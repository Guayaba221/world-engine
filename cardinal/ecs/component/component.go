package component

import (
	"errors"
	"fmt"
	"strconv"

	"pkg.world.dev/world-engine/cardinal/ecs"
	"pkg.world.dev/world-engine/cardinal/ecs/component_metadata"
	"pkg.world.dev/world-engine/cardinal/ecs/entity"
)

func Create(wCtx ecs.WorldContext, components ...component_metadata.Component) (entity.ID, error) {
	entities, err := CreateMany(wCtx, 1, components...)
	if err != nil {
		return 0, err
	}
	return entities[0], nil
}

func CreateMany(wCtx ecs.WorldContext, num int, components ...component_metadata.Component) ([]entity.ID, error) {
	if wCtx.IsReadOnly() {
		return nil, ecs.ErrorCannotModifyStateWithReadOnlyContext
	}
	world := wCtx.GetWorld()
	acc := make([]component_metadata.IComponentMetaData, 0, len(components))
	for _, comp := range components {
		c, err := world.GetComponentByName(comp.Name())
		if err != nil {
			return nil, err
		}
		acc = append(acc, c)
	}
	entityIds, err := world.StoreManager().CreateManyEntities(num, acc...)
	if err != nil {
		return nil, err
	}
	for _, id := range entityIds {
		for _, comp := range components {
			c, err := world.GetComponentByName(comp.Name())
			if err != nil {
				return nil, errors.New("Must register component before creating an entity")
			}
			err = world.StoreManager().SetComponentForEntity(c, id, comp)
			if err != nil {
				return nil, err
			}
		}
	}
	return entityIds, nil
}

// Removes a component from an entity
func RemoveComponentFrom[T component_metadata.Component](wCtx ecs.WorldContext, id entity.ID) error {
	if wCtx.IsReadOnly() {
		return ecs.ErrorCannotModifyStateWithReadOnlyContext
	}
	w := wCtx.GetWorld()
	var t T
	name := t.Name()
	c, err := w.GetComponentByName(name)
	if err != nil {
		return errors.New("Must register component")
	}
	return w.StoreManager().RemoveComponentFromEntity(c, id)
}

func AddComponentTo[T component_metadata.Component](wCtx ecs.WorldContext, id entity.ID) error {
	if wCtx.IsReadOnly() {
		return ecs.ErrorCannotModifyStateWithReadOnlyContext
	}
	w := wCtx.GetWorld()
	var t T
	name := t.Name()
	c, err := w.GetComponentByName(name)
	if err != nil {
		return errors.New("Must register component")
	}
	return w.StoreManager().AddComponentToEntity(c, id)
}

// GetComponent returns component data from the entity.
func GetComponent[T component_metadata.Component](wCtx ecs.WorldContext, id entity.ID) (comp *T, err error) {
	var t T
	name := t.Name()
	c, err := wCtx.GetWorld().GetComponentByName(name)
	if err != nil {
		return nil, errors.New("Must register component")
	}
	value, err := wCtx.StoreReader().GetComponentForEntity(c, id)
	if err != nil {
		return nil, err
	}
	t, ok := value.(T)
	if !ok {
		comp, ok = value.(*T)
		if !ok {
			return nil, fmt.Errorf("type assertion for component failed: %v to %v", value, c)
		}
	} else {
		comp = &t
	}

	return comp, nil
}

// Set sets component data to the entity.
func SetComponent[T component_metadata.Component](wCtx ecs.WorldContext, id entity.ID, component *T) error {
	if wCtx.IsReadOnly() {
		return ecs.ErrorCannotModifyStateWithReadOnlyContext
	}
	var t T
	name := t.Name()
	c, err := wCtx.GetWorld().GetComponentByName(name)
	if err != nil {
		return fmt.Errorf("%s is not registered, please register it before updating", t.Name())
	}
	err = wCtx.StoreManager().SetComponentForEntity(c, id, component)
	if err != nil {
		return err
	}
	wCtx.Logger().Debug().
		Str("entity_id", strconv.FormatUint(uint64(id), 10)).
		Str("component_name", c.Name()).
		Int("component_id", int(c.ID())).
		Msg("entity updated")
	return nil
}

func UpdateComponent[T component_metadata.Component](wCtx ecs.WorldContext, id entity.ID, fn func(*T) *T) error {
	if wCtx.IsReadOnly() {
		return ecs.ErrorCannotModifyStateWithReadOnlyContext
	}
	val, err := GetComponent[T](wCtx, id)
	if err != nil {
		return err
	}
	updatedVal := fn(val)
	return SetComponent[T](wCtx, id, updatedVal)
}