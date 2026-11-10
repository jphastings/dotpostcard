//go:build darwin

package fastfoto

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// IOKit exposes USB through COM-style vtables; slots are word indexes into them
// (IOUSBLib.h). IUNKNOWN_C_GUTS occupies 0-3.
const (
	vendorID, productID = 0x04b8, 0x016b

	slotQueryInterface = 1
	slotRelease        = 3

	// IOUSBDeviceInterface182
	slotDeviceOpen      = 8
	slotDeviceClose     = 9
	slotCreateIfaceIter = 28
	slotDeviceOpenSeize = 29

	// IOUSBInterfaceInterface190
	slotIfaceOpen   = 8
	slotIfaceClose  = 9
	slotNumEndpts   = 19
	slotPipeProps   = 26
	slotReadPipeTO  = 39
	slotWritePipeTO = 40

	endpointOut, endpointIn = 1, 2
	pipeBulk                = 2

	writeTimeoutMs = 5000
	readTimeoutMs  = 30000
	drainTimeoutMs = 200
	anyInterface   = 0xFFFF

	kIOReturnExclusiveAccess = 0xe00002c5
	utf8Encoding             = 0x08000100
)

var (
	loadOnce sync.Once
	loadErr  error

	ioServiceMatching           func(string) uintptr
	ioServiceGetMatchingService func(uint32, uintptr) uint32
	ioCreatePlugIn              func(uint32, uintptr, uintptr, *uintptr, *int32) int32
	ioDestroyPlugIn             func(uintptr) int32
	ioIteratorNext              func(uint32) uint32
	ioObjectRelease             func(uint32) int32
	cfRelease                   func(uintptr)
	cfStringCreate              func(uintptr, string, uint32) uintptr
	cfNumberCreate              func(uintptr, int32, *int32) uintptr
	cfDictSet                   func(uintptr, uintptr, uintptr)
	cfUUIDFromString            func(uintptr, uintptr) uintptr
	cfUUIDGetBytes              uintptr
)

// Go preempts threads with SIGURG, and IOKit reports a transfer interrupted by it as
// aborted. Blocking the signal during each call stops that.
func withoutPreemption(call func() int32) int32 {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	set, old := sigurgMask, uint32(0)
	pthreadSigmask(sigBlock, &set, &old)
	defer pthreadSigmask(sigSetmask, &old, nil)
	return call()
}

var pthreadSigmask func(how int32, set, old *uint32) int32

const (
	sigurgMask = uint32(1) << 15
	sigBlock   = int32(1)
	sigSetmask = int32(3)
)

func loadLibs() error {
	loadOnce.Do(func() {
		libSystem, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW)
		if err != nil {
			loadErr = err
			return
		}
		purego.RegisterLibFunc(&pthreadSigmask, libSystem, "pthread_sigmask")
		iokit, err := purego.Dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", purego.RTLD_NOW)
		if err != nil {
			loadErr = err
			return
		}
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW)
		if err != nil {
			loadErr = err
			return
		}
		purego.RegisterLibFunc(&ioServiceMatching, iokit, "IOServiceMatching")
		purego.RegisterLibFunc(&ioServiceGetMatchingService, iokit, "IOServiceGetMatchingService")
		purego.RegisterLibFunc(&ioCreatePlugIn, iokit, "IOCreatePlugInInterfaceForService")
		purego.RegisterLibFunc(&ioDestroyPlugIn, iokit, "IODestroyPlugInInterface")
		purego.RegisterLibFunc(&ioIteratorNext, iokit, "IOIteratorNext")
		purego.RegisterLibFunc(&ioObjectRelease, iokit, "IOObjectRelease")
		purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
		purego.RegisterLibFunc(&cfStringCreate, cf, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&cfNumberCreate, cf, "CFNumberCreate")
		purego.RegisterLibFunc(&cfDictSet, cf, "CFDictionarySetValue")
		purego.RegisterLibFunc(&cfUUIDFromString, cf, "CFUUIDCreateFromString")
		cfUUIDGetBytes, loadErr = purego.Dlsym(cf, "CFUUIDGetUUIDBytes")
	})
	return loadErr
}

// scratch holds out-parameters for IOKit calls. It lives on the heap and is pinned so
// the addresses handed to C stay valid.
type scratch struct {
	plug     uintptr
	score    int32
	iterator uint32
	request  [4]uint16
	count    uint8
	dir, num uint8
	kind     uint8
	interval uint8
	maxPkt   uint16
	size     uint32
}

type uuid struct{ ref, lo, hi uintptr }

func makeUUID(s string) uuid {
	str := cfStringCreate(0, s, utf8Encoding)
	defer cfRelease(str)
	ref := cfUUIDFromString(0, str)
	// CFUUIDBytes is a 16-byte struct returned in two registers.
	lo, hi, _ := purego.SyscallN(cfUUIDGetBytes, ref)
	return uuid{ref, lo, hi}
}

// read loads the word at a C address. It goes via a stored pointer because vet rejects
// converting a uintptr, but these addresses belong to IOKit, not the Go heap.
func read(addr uintptr) uintptr {
	return *(*uintptr)(*(*unsafe.Pointer)(unsafe.Pointer(&addr)))
}

// callSlot invokes slot n of COM object obj (a pointer to a pointer to a vtable).
func callSlot(obj uintptr, slot int, args ...uintptr) int32 {
	fn := read(read(obj) + uintptr(slot)*unsafe.Sizeof(uintptr(0)))
	r, _, _ := purego.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return int32(r)
}

// transferError is for failures once a session is running, where the code means nothing
// to the user: the scanner has gone quiet.
func transferError(what string, r int32) error {
	if uint32(r) == kIOReturnExclusiveAccess {
		return ErrBusyUSB
	}
	return fmt.Errorf("%w (%s: IOReturn 0x%08x)", ErrNotResponding, what, uint32(r))
}

func ioError(what string, r int32) error {
	if uint32(r) == kIOReturnExclusiveAccess {
		return ErrBusyUSB
	}
	return fmt.Errorf("%s: IOReturn 0x%08x", what, uint32(r))
}

type iokitTransport struct {
	device, iface    uintptr
	deviceOpened     bool
	pipeOut, pipeIn  uintptr
	scratch          *scratch
	scratchPin       runtime.Pinner
	service, ifaceSv uint32
	iterator         uint32
}

// OpenUSB opens the FF-680W's bulk pipes through IOKit.
func OpenUSB() (Transport, error) {
	if err := loadLibs(); err != nil {
		return nil, err
	}
	t := &iokitTransport{scratch: new(scratch)}
	t.scratchPin.Pin(t.scratch)
	if err := t.open(); err != nil {
		t.Close()
		return nil, err
	}
	return t, nil
}

func (t *iokitTransport) userClient(service uint32, typeID, ifaceID, plugID uuid) (uintptr, error) {
	sc := t.scratch
	if r := ioCreatePlugIn(service, typeID.ref, plugID.ref, &sc.plug, &sc.score); r != 0 {
		return 0, ioError("IOCreatePlugInInterfaceForService", r)
	}
	plug := sc.plug
	defer ioDestroyPlugIn(plug)
	sc.plug = 0
	if r := callSlot(plug, slotQueryInterface, ifaceID.lo, ifaceID.hi, uintptr(unsafe.Pointer(&sc.plug))); r != 0 || sc.plug == 0 {
		return 0, ioError("QueryInterface", r)
	}
	return sc.plug, nil
}

func (t *iokitTransport) open() error {
	// These constants come from IOUSBLib.h / IOCFPlugIn.h; CFUUIDGetConstantUUIDWithBytes
	// takes 16 byte arguments that purego can't pass reliably, so they're built from strings.
	deviceClient := makeUUID("9dc7b780-9ec0-11d4-a54f-000a27052861")
	ifaceClient := makeUUID("2d9786c6-9ef3-11d4-ad51-000a27052861")
	plugID := makeUUID("c244e858-109c-11d4-91d4-0050e4c6426f")
	device182 := makeUUID("152fc496-4891-11d5-9d52-000a27801e86")
	iface190 := makeUUID("8fdb8455-74a6-11d6-97b1-003065d3608e")
	defer func() {
		for _, u := range []uuid{deviceClient, ifaceClient, plugID, device182, iface190} {
			cfRelease(u.ref)
		}
	}()

	match := ioServiceMatching("IOUSBHostDevice")
	for key, value := range map[string]int32{"idVendor": vendorID, "idProduct": productID} {
		k, n := cfStringCreate(0, key, utf8Encoding), cfNumberCreate(0, 3, &value)
		cfDictSet(match, k, n)
		cfRelease(k)
		cfRelease(n)
	}
	t.service = ioServiceGetMatchingService(0, match) // consumes match
	if t.service == 0 {
		return ErrNoScanner
	}

	var err error
	if t.device, err = t.userClient(t.service, deviceClient, device182, plugID); err != nil {
		return fmt.Errorf("opening scanner: %w", err)
	}
	// Another driver may already hold the device open; the interface open below is what decides.
	t.deviceOpened = callSlot(t.device, slotDeviceOpenSeize) == 0

	sc := t.scratch
	sc.request = [4]uint16{anyInterface, anyInterface, anyInterface, anyInterface}
	if r := callSlot(t.device, slotCreateIfaceIter, uintptr(unsafe.Pointer(&sc.request)), uintptr(unsafe.Pointer(&sc.iterator))); r != 0 {
		return ioError("CreateInterfaceIterator", r)
	}
	t.iterator = sc.iterator
	if t.ifaceSv = ioIteratorNext(t.iterator); t.ifaceSv == 0 {
		return errors.New("the scanner has no USB interface yet; switch it off and on again")
	}
	if t.iface, err = t.userClient(t.ifaceSv, ifaceClient, iface190, plugID); err != nil {
		return fmt.Errorf("opening scanner interface: %w", err)
	}
	if r := callSlot(t.iface, slotIfaceOpen); r != 0 {
		t.iface, _ = releaseAndZero(t.iface)
		return ioError("USBInterfaceOpen", r)
	}
	return t.findPipes()
}

func releaseAndZero(obj uintptr) (uintptr, int32) { return 0, callSlot(obj, slotRelease) }

func (t *iokitTransport) findPipes() error {
	sc := t.scratch
	if r := callSlot(t.iface, slotNumEndpts, uintptr(unsafe.Pointer(&sc.count))); r != 0 {
		return ioError("GetNumEndpoints", r)
	}
	for ref := uintptr(1); ref <= uintptr(sc.count); ref++ {
		if callSlot(t.iface, slotPipeProps, ref,
			uintptr(unsafe.Pointer(&sc.dir)), uintptr(unsafe.Pointer(&sc.num)), uintptr(unsafe.Pointer(&sc.kind)),
			uintptr(unsafe.Pointer(&sc.maxPkt)), uintptr(unsafe.Pointer(&sc.interval))) != 0 {
			continue
		}
		switch {
		case sc.kind == pipeBulk && sc.dir == 0 && sc.num == endpointOut:
			t.pipeOut = ref
		case sc.kind == pipeBulk && sc.dir == 1 && sc.num == endpointIn:
			t.pipeIn = ref
		}
	}
	if t.pipeOut == 0 || t.pipeIn == 0 {
		return errors.New("the scanner's bulk pipes weren't found; switch it off and on again")
	}
	return nil
}

func (t *iokitTransport) Write(p []byte) error {
	var pin runtime.Pinner
	pin.Pin(&p[0])
	defer pin.Unpin()
	r := withoutPreemption(func() int32 {
		return callSlot(t.iface, slotWritePipeTO, t.pipeOut, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)), writeTimeoutMs, writeTimeoutMs)
	})
	if r != 0 {
		return transferError("write to scanner", r)
	}
	return nil
}

func (t *iokitTransport) Read(p []byte) (int, error) {
	var pin runtime.Pinner
	pin.Pin(&p[0])
	defer pin.Unpin()
	t.scratch.size = uint32(len(p))
	r := withoutPreemption(func() int32 {
		return callSlot(t.iface, slotReadPipeTO, t.pipeIn, uintptr(unsafe.Pointer(&p[0])), uintptr(unsafe.Pointer(&t.scratch.size)), readTimeoutMs, readTimeoutMs)
	})
	if r != 0 {
		return 0, transferError("read from scanner", r)
	}
	return int(t.scratch.size), nil
}

func (t *iokitTransport) Drain() {
	buf := make([]byte, 1<<20)
	var pin runtime.Pinner
	pin.Pin(&buf[0])
	defer pin.Unpin()
	// Stale image blocks can be a mebibyte each; the cap only guards against a scanner that never goes quiet.
	for range 1000 {
		t.scratch.size = uint32(len(buf))
		r := withoutPreemption(func() int32 {
			return callSlot(t.iface, slotReadPipeTO, t.pipeIn, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&t.scratch.size)), drainTimeoutMs, drainTimeoutMs)
		})
		if r != 0 {
			return
		}
	}
}

func (t *iokitTransport) Close() error {
	if t.iface != 0 {
		callSlot(t.iface, slotIfaceClose)
		callSlot(t.iface, slotRelease)
	}
	if t.iterator != 0 {
		ioObjectRelease(t.iterator)
	}
	if t.ifaceSv != 0 {
		ioObjectRelease(t.ifaceSv)
	}
	if t.device != 0 {
		if t.deviceOpened {
			callSlot(t.device, slotDeviceClose)
		}
		callSlot(t.device, slotRelease)
	}
	if t.service != 0 {
		ioObjectRelease(t.service)
	}
	t.scratchPin.Unpin()
	*t = iokitTransport{}
	return nil
}
