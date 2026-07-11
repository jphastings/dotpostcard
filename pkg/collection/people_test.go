package collection

import (
	"testing"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

func TestPeopleMergesRolesAcrossCards(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-sender", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones"}
		m.Recipient = types.Person{}
		m.Context.Author = types.Person{}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	other, otherFilename := encodeCard(t, "card-recipient", func(m *types.Metadata) {
		m.Sender = types.Person{}
		m.Recipient = types.Person{Name: "Claire Jones"}
		m.Context.Author = types.Person{}
	})
	_, err = col.AddWebPostcard(otherFilename, other)
	assert.NoError(t, err)

	people, err := col.People()
	assert.NoError(t, err)
	assert.Len(t, people, 1)
	assert.Equal(t, "Claire Jones", people[0].Name)
	assert.Equal(t, []string{"from", "to"}, people[0].Roles)
}

func TestPeopleSameNameDifferentURIsAreDistinct(t *testing.T) {
	col := mustCreate(t)

	dataA, filenameA := encodeCard(t, "card-a", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones", Uri: "https://claire-a.example.com"}
		m.Recipient = types.Person{}
		m.Context.Author = types.Person{}
	})
	_, err := col.AddWebPostcard(filenameA, dataA)
	assert.NoError(t, err)

	dataB, filenameB := encodeCard(t, "card-b", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones", Uri: "https://claire-b.example.com"}
		m.Recipient = types.Person{}
		m.Context.Author = types.Person{}
	})
	_, err = col.AddWebPostcard(filenameB, dataB)
	assert.NoError(t, err)

	people, err := col.People()
	assert.NoError(t, err)
	assert.Len(t, people, 2)
	assert.Equal(t, "https://claire-a.example.com", people[0].Uri)
	assert.Equal(t, "https://claire-b.example.com", people[1].Uri)
}

func TestPeopleSkipsEmptyPerson(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-no-sender", func(m *types.Metadata) {
		m.Sender = types.Person{}
		m.Recipient = types.Person{}
		m.Context.Author = types.Person{}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	people, err := col.People()
	assert.NoError(t, err)
	assert.Empty(t, people, "a card with no sender, recipient or collector at all should contribute no people")
}

func TestPeopleCollectorRole(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-collected", func(m *types.Metadata) {
		m.Sender = types.Person{}
		m.Recipient = types.Person{}
		m.Context.Author = types.Person{Name: "Carol Smith"}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	people, err := col.People()
	assert.NoError(t, err)
	assert.Len(t, people, 1)
	assert.Equal(t, "Carol Smith", people[0].Name)
	assert.Equal(t, []string{"collector"}, people[0].Roles)
}

func TestPeopleDeterministicOrder(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-one", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Zara Ahmed"}
		m.Recipient = types.Person{Name: "Amy Brown"}
		m.Context.Author = types.Person{Name: "Mo Chen"}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	people, err := col.People()
	assert.NoError(t, err)

	var names []string
	for _, p := range people {
		names = append(names, p.Name)
	}
	assert.Equal(t, []string{"Amy Brown", "Mo Chen", "Zara Ahmed"}, names)
}
