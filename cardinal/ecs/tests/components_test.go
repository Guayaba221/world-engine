package tests

import (
	"testing"

	"github.com/argus-labs/world-engine/cardinal/ecs"
	"github.com/argus-labs/world-engine/cardinal/ecs/component"
	"github.com/argus-labs/world-engine/cardinal/ecs/inmem"
	storage2 "github.com/argus-labs/world-engine/cardinal/ecs/storage"
	"gotest.tools/v3/assert"
)

func TestComponents(t *testing.T) {
	type ComponentData struct {
		ID string
	}
	var (
		ca = storage2.NewMockComponentType(ComponentData{}, ComponentData{ID: "foo"})
		cb = storage2.NewMockComponentType(ComponentData{}, ComponentData{ID: "bar"})
	)

	components := storage2.NewComponents(storage2.NewComponentsSliceStorage(), storage2.NewComponentIndexMap())

	tests := []*struct {
		layout  *storage2.Layout
		archIdx storage2.ArchetypeIndex
		compIdx storage2.ComponentIndex
		ID      string
	}{
		{
			storage2.NewLayout([]component.IComponentType{ca}),
			0,
			0,
			"a",
		},
		{
			storage2.NewLayout([]component.IComponentType{ca, cb}),
			1,
			1,
			"b",
		},
	}

	for _, tt := range tests {
		var err error
		tt.compIdx, err = components.PushComponents(tt.layout.Components(), tt.archIdx)
		assert.NilError(t, err)
	}

	for _, tt := range tests {
		for _, comp := range tt.layout.Components() {
			st := components.Storage(comp)
			ok, err := st.Contains(tt.archIdx, tt.compIdx)
			assert.NilError(t, err)
			if !ok {
				t.Errorf("storage should contain the component at %d, %d", tt.archIdx, tt.compIdx)
			}
			bz, _ := st.Component(tt.archIdx, tt.compIdx)
			dat, err := storage2.Decode[ComponentData](bz)
			assert.NilError(t, err)
			dat.ID = tt.ID

			compBz, err := storage2.Encode(dat)
			assert.NilError(t, err)

			err = st.SetComponent(tt.archIdx, tt.compIdx, compBz)
			assert.NilError(t, err)
		}
	}

	target := tests[0]
	storage := components.Storage(ca)

	srcArchIdx := target.archIdx
	var dstArchIdx storage2.ArchetypeIndex = 1

	assert.NilError(t, storage.MoveComponent(srcArchIdx, target.compIdx, dstArchIdx))
	assert.NilError(t, components.Move(srcArchIdx, dstArchIdx))

	ok, err := storage.Contains(srcArchIdx, target.compIdx)
	if ok {
		t.Errorf("storage should not contain the component at %d, %d", target.archIdx, target.compIdx)
	}
	if idx, _, _ := components.ComponentIndices.ComponentIndex(srcArchIdx); idx != -1 {
		t.Errorf("component Index should be -1 at %d but %d", srcArchIdx, idx)
	}

	newCompIdx, _, _ := components.ComponentIndices.ComponentIndex(dstArchIdx)

	ok, err = storage.Contains(dstArchIdx, newCompIdx)
	if !ok {
		t.Errorf("storage should contain the component at %d, %d", dstArchIdx, target.compIdx)
	}

	bz, _ := storage.Component(dstArchIdx, newCompIdx)
	dat, err := storage2.Decode[ComponentData](bz)
	assert.NilError(t, err)
	if dat.ID != target.ID {
		t.Errorf("component should have ID '%s', got ID '%s'", target.ID, dat.ID)
	}
}

func TestErrorWhenAccessingComponentNotOnEntity(t *testing.T) {
	world := inmem.NewECSWorldForTest(t)
	foundComp := ecs.NewComponentType[string]()
	notFoundComp := ecs.NewComponentType[string]()
	world.RegisterComponents(foundComp, notFoundComp)

	id, err := world.Create(foundComp)
	assert.NilError(t, err)
	_, err = notFoundComp.Get(world, id)
	assert.ErrorIs(t, err, storage2.ErrorComponentNotOnEntity)
}
