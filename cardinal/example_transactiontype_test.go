package cardinal_test

import (
	"errors"

	"pkg.world.dev/world-engine/cardinal"
)

type MovePlayerMsg struct {
	DeltaX int
	DeltaY int
}

type MovePlayerResult struct {
	FinalX int
	FinalY int
}

var MoveTx = cardinal.NewTransactionType[MovePlayerMsg, MovePlayerResult]("move-player")

func ExampleTransactionType() {
	world, err := cardinal.NewMockWorld()
	if err != nil {
		panic(err)
	}

	world.RegisterSystems(func(world *cardinal.World, queue *cardinal.TransactionQueue, logger *cardinal.Logger) error {
		for _, tx := range MoveTx.In(queue) {
			msg := tx.Value()
			// handle the transaction
			// ...

			// save the result
			MoveTx.SetResult(world, tx.Hash(), MovePlayerResult{
				FinalX: msg.DeltaX,
				FinalY: msg.DeltaY,
			})

			// optionally, add an error to the transaction
			MoveTx.AddError(world, tx.Hash(), errors.New("some error"))
		}
		return nil
	})

	// The above system will be called during each game tick.
}