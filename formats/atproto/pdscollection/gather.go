package pdscollection

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdsbundle"
	"github.com/jphastings/dotpostcard/formats/atproto/pdsupload"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/inprogress"
	"github.com/jphastings/dotpostcard/pkg/collection"
)

const collectionSuffix = ".postcards"

// fetchConcurrency bounds how many cards of an atproto collection are downloaded at once.
const fetchConcurrency = 6

// webFileSuffixes mirrors the suffixes formats/web recognises as a web-format postcard file.
var webFileSuffixes = []string{".postcard.webp", ".postcard.jpg", ".postcard.jpeg", ".postcard.png", ".postcard"}

var blobExtensions = map[string]string{"image/webp": "webp", "image/jpeg": "jpg", "image/png": "png"}

// BuildOptions configures how cards from sources are stored, for Create and Add.
//
// A source is a web-format postcard file, a *.postcards file (all its cards), a directory
// (non-recursively: its postcard and *.postcards files), or an at:// URI of a postcard record
// or of a collection record (all its cards).
type BuildOptions struct {
	// Quality, if set, decodes each card and re-encodes it at that quality. Unset stores every
	// card's bytes untouched.
	Quality *formats.Quality
	// PDSHost and PLCHost override identity resolution for at:// sources, as for atproto.Anonymous.
	PDSHost, PLCHost string
	// Warn, if set, is told about name clashes between sources, and about each card from
	// atproto whose record and image metadata disagree.
	Warn func(string)
	// OnCard, if set, is told the name of each card as it is stored.
	OnCard func(name string)
	// Cached, if set, is asked for an atproto card's image by its sha256 before the image is
	// downloaded; a hit skips the download, unless the bytes don't hash to sum. It may be called
	// concurrently.
	Cached func(sum [sha256.Size]byte) (data []byte, ok bool)
	// OnRecord, if set, is told, after each card from an atproto postcard record is stored, the
	// record's at-uri and the name its card is stored under. done counts the records of the
	// source handled so far, of total (1 of 1 for a lone postcard record).
	OnRecord func(uri, name string, done, total int)
	// Stop, if set, cancels the build between cards; the build then fails with its error.
	Stop context.Context
}

// CreateOptions configures Create.
type CreateOptions struct {
	BuildOptions
	// Title names the collection. Empty inherits the title of the sole source, if that is a
	// collection.
	Title string
	// WriteFile writes <title key>.postcards into OutDir (the current directory if empty).
	WriteFile bool
	OutDir    string
	Overwrite bool
	// Client, if set, publishes the collection as that account, per Publish.
	Client *atproto.Client
	// Publish carries the dry-run and progress settings of the publish. A dry run writes
	// neither the file nor any record.
	Publish PublishOptions
}

// CreateResult reports what Create did. Path is empty when no file was written. Published is
// nil unless the collection was published.
type CreateResult struct {
	Path      string
	Cards     []string
	Published *PublishResult
}

// Create builds a collection from sources, then writes it to a file and/or publishes it as
// opts asks. The file and the record are both keyed by the title, so creating the same title
// again replaces the file (with Overwrite) and updates the same record.
//
// A new file, with the title given up front, is written at its final name as it is built (shown
// as in progress where the OS can) and removed if the build fails. Replacing a file is built
// aside, so the old one survives a failure.
func Create(sources []string, opts CreateOptions) (CreateResult, error) {
	var result CreateResult
	writeFile := opts.WriteFile && !opts.Publish.DryRun

	target := ""
	title := opts.Title
	if title != "" {
		var err error
		if target, err = opts.targetPath(title); err != nil {
			return result, err
		}
	}

	inPlace := writeFile && target != "" && !exists(target)
	var path string
	if inPlace {
		path = target
	} else {
		stageParent := ""
		if writeFile {
			stageParent = cmp.Or(opts.OutDir, ".")
		}
		// Inside the output directory, so the final rename never crosses filesystems.
		stage, err := os.MkdirTemp(stageParent, ".postcards-build-*")
		if err != nil {
			return result, err
		}
		defer os.RemoveAll(stage)
		path = filepath.Join(stage, "collection"+collectionSuffix)
	}

	var coll *collection.Collection
	built := false
	if inPlace {
		defer func() {
			if !built {
				if coll != nil {
					coll.Close()
				}
				os.Remove(path)
			}
		}()
	}
	coll, err := collection.Create(path)
	if err != nil {
		return result, err
	}
	defer coll.Close()

	b := newBuilder(coll, opts.BuildOptions)
	if inPlace {
		inprogress.Mark(path)
		b.step = func(done, total int) { inprogress.SetFraction(path, float64(done)/float64(total)) }
	}
	sourceTitle, err := b.addSources(sources, target)
	if err != nil {
		return result, err
	}
	if title == "" {
		if len(sources) == 1 {
			title = sourceTitle
		}
		if title == "" {
			return result, errors.New("a title is needed: pass --title (only a single collection source supplies its own)")
		}
		if target, err = opts.targetPath(title); err != nil {
			return result, err
		}
	}
	if err := coll.SetTitle(title); err != nil {
		return result, err
	}
	result.Cards = b.names
	if err := coll.Close(); err != nil {
		return result, err
	}

	if inPlace {
		inprogress.Clear(path)
		built = true
		result.Path = path
	} else if writeFile {
		if err := os.Rename(path, target); err != nil {
			return result, err
		}
		path, result.Path = target, target
	}

	if opts.Client != nil {
		published, err := publishFile(opts.Client, path, opts.Publish)
		if err != nil {
			return result, err
		}
		result.Published = &published
	}
	return result, nil
}

// Add adds the cards of sources to the collection at path, returning their names. A card
// replaces any already in the collection with the same name.
func Add(path string, sources []string, opts BuildOptions) ([]string, error) {
	coll, err := collection.Open(path)
	if err != nil {
		return nil, err
	}
	defer coll.Close()

	b := newBuilder(coll, opts)
	_, err = b.addSources(sources, path)
	return b.names, err
}

// Sync makes the collection file at path, created if it's missing, hold the cards of the atproto
// collection at uri under the collection's title, committing each card as it arrives so readers
// see the file grow. Cards the record no longer lists are removed once every card has arrived; a
// failure leaves the cards committed so far, and running Sync again carries on.
func Sync(path, uri string, opts BuildOptions) error {
	if _, nsid, _, err := atproto.ParseATURI(uri); err != nil {
		return err
	} else if nsid != atproto.CollectionRecordType {
		return fmt.Errorf("%q is not an %s record URI", uri, atproto.CollectionRecordType)
	}

	open := collection.Create
	if exists(path) {
		open = collection.Open
	}
	coll, err := open(path)
	if err != nil {
		return err
	}
	defer coll.Close()

	b := newBuilder(coll, opts)
	b.onTitle = coll.SetTitle
	if _, err := b.addRemote(uri); err != nil {
		return err
	}

	cards, err := coll.List()
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	for _, name := range b.names {
		kept[name] = true
	}
	for _, card := range cards {
		if !kept[card.Name] {
			if err := coll.Remove(card.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (o CreateOptions) targetPath(title string) (string, error) {
	if !o.WriteFile {
		return "", nil
	}
	path := filepath.Join(o.OutDir, titleKey(title)+collectionSuffix)
	if _, err := os.Stat(path); err == nil && !o.Overwrite {
		return "", fmt.Errorf("%s already exists (pass --overwrite to replace it)", path)
	}
	return path, nil
}

func publishFile(client *atproto.Client, path string, opts PublishOptions) (PublishResult, error) {
	coll, err := collection.Open(path)
	if err != nil {
		return PublishResult{}, err
	}
	defer coll.Close()
	return Publish(client, coll, opts)
}

type sourceCard struct {
	// filename is a base name that carries the card's name and image type.
	filename string
	data     []byte
	// record is set for cards from atproto.
	record *atproto.Record
}

type contentKey struct {
	name string
	sum  [sha256.Size]byte
}

type builder struct {
	coll *collection.Collection
	opts BuildOptions
	// clients is guarded by clientsMu, as cards are fetched concurrently.
	clientsMu sync.Mutex
	clients   map[string]*atproto.Client
	taken     map[string]bool
	seen      map[contentKey]string
	names     []string
	// step, if set, is told how many sources are done of how many in total.
	step func(done, total int)
	// onTitle, if set, is told the title of an atproto collection as soon as its record is fetched.
	onTitle func(title string) error
}

func newBuilder(coll *collection.Collection, opts BuildOptions) *builder {
	return &builder{coll: coll, opts: opts, clients: map[string]*atproto.Client{}, taken: map[string]bool{}, seen: map[contentKey]string{}}
}

// addSources stores the cards of each source. When there's one source that is a collection,
// its title is returned. A directory source skips skipPath, so a collection can be rebuilt
// from the directory its previous version sits in.
func (b *builder) addSources(sources []string, skipPath string) (title string, err error) {
	items, err := expandDirectories(sources, skipPath)
	if err != nil {
		return "", err
	}
	// ponytail: a *.postcards or at:// collection counts as one step however many cards it
	// holds; count their cards up front to make the progress even.
	for i, item := range items {
		itemTitle, err := b.addItem(item)
		if err != nil {
			return "", err
		}
		if len(sources) == 1 && item == sources[0] {
			title = itemTitle
		}
		if b.step != nil {
			b.step(i+1, len(items))
		}
	}
	return title, nil
}

// expandDirectories replaces each directory with its source files, skipping skipPath.
func expandDirectories(sources []string, skipPath string) ([]string, error) {
	var items []string
	for _, source := range sources {
		if strings.HasPrefix(source, "at://") {
			items = append(items, source)
			continue
		}
		info, err := os.Stat(source)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			items = append(items, source)
			continue
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			path := filepath.Join(source, entry.Name())
			if entry.IsDir() || !isSourceFile(path) || (skipPath != "" && samePath(path, skipPath)) {
				continue
			}
			items = append(items, path)
		}
	}
	return items, nil
}

func (b *builder) addItem(item string) (string, error) {
	if strings.HasPrefix(item, "at://") {
		return b.addRemote(item)
	}
	return b.addFile(item)
}

func (b *builder) addFile(path string) (string, error) {
	if strings.HasSuffix(path, collectionSuffix) {
		return b.addCollectionFile(path)
	}
	if !isSourceFile(path) {
		return "", fmt.Errorf("%s is not a web-format postcard or %s file", path, collectionSuffix)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	_, err = b.add(sourceCard{filename: filepath.Base(path), data: data})
	return "", err
}

func (b *builder) addCollectionFile(path string) (string, error) {
	source, err := collection.OpenReadOnly(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}
	defer source.Close()

	title, err := source.Title()
	if err != nil {
		return "", err
	}
	cards, err := source.List()
	if err != nil {
		return "", err
	}
	for _, card := range cards {
		data, mimetype, err := source.CardData(card.Name)
		if err != nil {
			return "", err
		}
		filename, err := cardFilename(card.Name, mimetype)
		if err != nil {
			return "", err
		}
		if _, err := b.add(sourceCard{filename: filename, data: data}); err != nil {
			return "", err
		}
	}
	return title, nil
}

func (b *builder) addRemote(uri string) (string, error) {
	authority, nsid, rkey, err := atproto.ParseATURI(uri)
	if err != nil {
		return "", err
	}

	switch nsid {
	case atproto.RecordType:
		client, err := b.client(authority)
		if err != nil {
			return "", err
		}
		record, cid, err := client.GetRecord(client.DID(), rkey)
		if err != nil {
			return "", fmt.Errorf("fetching postcard %s: %w", uri, err)
		}
		data, err := b.imageOf(client, record, uri)
		if err != nil {
			return "", err
		}
		return "", b.addFetched(fetchedCard{uri: uri, cid: cid, record: record, data: data}, 1, 1)
	case atproto.CollectionRecordType:
		published, err := atproto.FetchCollectionRecord(uri, b.opts.PDSHost, b.opts.PLCHost)
		if err != nil {
			return "", err
		}
		if b.onTitle != nil {
			if err := b.onTitle(published.Title); err != nil {
				return "", err
			}
		}
		if err := b.addCollectionCards(published.Postcards); err != nil {
			return "", err
		}
		return published.Title, nil
	}
	return "", fmt.Errorf("%s is neither an %s nor an %s record", uri, atproto.RecordType, atproto.CollectionRecordType)
}

// fetchedCard is a postcard record with its image, ready to be stored.
type fetchedCard struct {
	uri, cid string
	record   atproto.Record
	data     []byte
	err      error
}

// addCollectionCards fetches the referenced postcards with at most fetchConcurrency in flight,
// but stores them one at a time in the order referenced, so card names don't depend on timing.
func (b *builder) addCollectionCards(refs []atproto.StrongRef) error {
	authorities := make([]string, len(refs))
	for i, ref := range refs {
		authority, nsid, _, err := atproto.ParseATURI(ref.URI)
		if err != nil {
			return err
		}
		if nsid != atproto.RecordType {
			return fmt.Errorf("%q is not an %s record URI", ref.URI, atproto.RecordType)
		}
		authorities[i] = authority
	}

	// Each result has its own buffered channel so a job never blocks, even once we've given up on it.
	results := make([]chan fetchedCard, len(refs))
	launched := 0
	launch := func() {
		i := launched
		launched++
		results[i] = make(chan fetchedCard, 1)
		go func() { results[i] <- b.fetchCard(refs[i], authorities[i]) }()
	}
	for launched < min(fetchConcurrency, len(refs)) {
		launch()
	}

	for i := range refs {
		card := <-results[i]
		if card.err != nil {
			return card.err
		}
		if launched < len(refs) {
			launch()
		}
		if err := b.addFetched(card, i+1, len(refs)); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) fetchCard(ref atproto.StrongRef, authority string) fetchedCard {
	card := fetchedCard{uri: ref.URI, cid: ref.CID}
	client, err := b.client(authority)
	if err != nil {
		card.err = err
		return card
	}
	_, _, rkey, _ := atproto.ParseATURI(ref.URI)
	// A record whose CID has moved on since it was referenced is still fetched, as records can be updated.
	if _, err := client.GetRecordIn(client.DID(), atproto.RecordType, rkey, &card.record); err != nil {
		card.err = fmt.Errorf("fetching postcard %s: %w", ref.URI, err)
		return card
	}
	card.data, card.err = b.imageOf(client, card.record, ref.URI)
	return card
}

// imageOf returns the record's image, from the Cached hook if it has bytes that really are the image.
func (b *builder) imageOf(client *atproto.Client, record atproto.Record, uri string) ([]byte, error) {
	if b.opts.Cached != nil {
		if sum, err := record.Image.SHA256(); err == nil {
			if data, ok := b.opts.Cached(sum); ok && sha256.Sum256(data) == sum {
				return data, nil
			}
		}
	}
	data, err := client.GetBlob(client.DID(), record.Image.Ref.Link)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", uri, err)
	}
	return data, nil
}

func (b *builder) addFetched(card fetchedCard, done, total int) error {
	filename, err := cardFilename(pdsbundle.NameFromRecord(card.record, card.cid), card.record.Image.MimeType)
	if err != nil {
		return fmt.Errorf("%s: %w", card.uri, err)
	}
	name, err := b.add(sourceCard{filename: filename, data: card.data, record: &card.record})
	if err != nil {
		return fmt.Errorf("%s: %w", card.uri, err)
	}
	if b.opts.OnRecord != nil {
		b.opts.OnRecord(card.uri, name, done, total)
	}
	return nil
}

func (b *builder) client(authority string) (*atproto.Client, error) {
	b.clientsMu.Lock()
	defer b.clientsMu.Unlock()
	if client, ok := b.clients[authority]; ok {
		return client, nil
	}
	client, err := atproto.Anonymous(authority, b.opts.PDSHost, b.opts.PLCHost)
	if err != nil {
		return nil, err
	}
	b.clients[authority] = client
	return client, nil
}

// add stores card unless an identical card of the same name was already stored in this run;
// a different card with a taken name is stored under a suffixed one. It returns the name the
// card is stored under, which for an identical duplicate is the existing card's.
func (b *builder) add(card sourceCard) (string, error) {
	if b.opts.Stop != nil && b.opts.Stop.Err() != nil {
		return "", context.Cause(b.opts.Stop)
	}
	baseName := web.CardNameFromPath(card.filename)
	key := contentKey{baseName, sha256.Sum256(card.data)}
	if name, ok := b.seen[key]; ok {
		return name, nil
	}

	name := uniqueName(baseName, b.taken)
	b.seen[key] = name
	if name != baseName && b.opts.Warn != nil {
		b.opts.Warn(fmt.Sprintf("%s: the name is taken by another card, so this one is stored as %s", baseName, name))
	}
	filename := name + strings.TrimPrefix(card.filename, baseName)
	data := card.data

	if card.record != nil && b.opts.Warn != nil {
		if err := warnOnDifference(b.opts.Warn, name, baseName, *card.record, data); err != nil {
			return "", err
		}
	}
	if b.opts.Quality != nil {
		var err error
		if filename, data, err = reencode(filename, data, *b.opts.Quality); err != nil {
			return "", fmt.Errorf("re-encoding %s: %w", name, err)
		}
	}

	if _, err := b.coll.AddWebPostcard(filename, data); err != nil {
		return "", fmt.Errorf("adding %s: %w", name, err)
	}
	b.names = append(b.names, name)
	if b.opts.OnCard != nil {
		b.opts.OnCard(name)
	}
	return name, nil
}

func reencode(filename string, data []byte, quality formats.Quality) (string, []byte, error) {
	pc, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), filename).Decode(formats.DecodeOptions{})
	if err != nil {
		return "", nil, err
	}
	filename, _, data, err = pdsupload.EncodeImage(pc, &formats.EncodeOptions{Quality: quality})
	return filename, data, err
}

func cardFilename(name, mimetype string) (string, error) {
	ext, ok := blobExtensions[mimetype]
	if !ok {
		return "", fmt.Errorf("%s: unsupported image type %q", name, mimetype)
	}
	return name + ".postcard." + ext, nil
}

func isSourceFile(path string) bool {
	if strings.HasSuffix(path, collectionSuffix) {
		return true
	}
	for _, suffix := range webFileSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && absA == absB
}

func uniqueName(name string, taken map[string]bool) string {
	candidate := name
	for n := 2; taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", name, n)
	}
	taken[candidate] = true
	return candidate
}

// baseName is the name before any clash suffix: it's what the record's title derives from.
func warnOnDifference(warn func(string), name, baseName string, record atproto.Record, data []byte) error {
	pc, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), name).Decode(formats.DecodeOptions{})
	if err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}
	if diffs := atproto.Diff(atproto.FromMetadata(pc.Meta, record.Image), record, baseName); len(diffs) > 0 {
		warn(fmt.Sprintf("%s: record and image metadata differ: %s", name, strings.Join(diffs, ", ")))
	}
	return nil
}
