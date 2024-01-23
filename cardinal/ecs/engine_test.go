package ecs_test

import (
	"context"
	"encoding/json"
	"errors"
	"pkg.world.dev/world-engine/cardinal/shard/adapter"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"pkg.world.dev/world-engine/cardinal"
	"pkg.world.dev/world-engine/cardinal/txpool"
	"pkg.world.dev/world-engine/evm/x/shard/types"

	"pkg.world.dev/world-engine/cardinal/testutils"

	"pkg.world.dev/world-engine/sign"

	"pkg.world.dev/world-engine/assert"

	"pkg.world.dev/world-engine/cardinal/ecs"
)

func TestCanWaitForNextTick(t *testing.T) {
	engine := testutils.NewTestFixture(t, nil).Engine
	startTickCh := make(chan time.Time)
	doneTickCh := make(chan uint64)
	assert.NilError(t, engine.LoadGameState())
	engine.StartGameLoop(context.Background(), startTickCh, doneTickCh)

	// Make sure the game can tick
	startTickCh <- time.Now()
	<-doneTickCh

	waitForNextTickDone := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			success := engine.WaitForNextTick()
			assert.Check(t, success)
		}
		close(waitForNextTickDone)
	}()

	for {
		select {
		case startTickCh <- time.Now():
			<-doneTickCh
		case <-waitForNextTickDone:
			// The above goroutine successfully waited multiple times
			return
		}
	}
}

func TestWaitForNextTickReturnsFalseWhenEngineIsShutDown(t *testing.T) {
	engine := testutils.NewTestFixture(t, nil).Engine
	startTickCh := make(chan time.Time)
	doneTickCh := make(chan uint64)
	assert.NilError(t, engine.LoadGameState())
	engine.StartGameLoop(context.Background(), startTickCh, doneTickCh)

	// Make sure the game can tick
	startTickCh <- time.Now()
	<-doneTickCh

	waitForNextTickDone := make(chan struct{})
	go func() {
		// continually spin here waiting for next tick. One of these must fail before
		// the test times out for this test to pass
		for engine.WaitForNextTick() {
		}
		close(waitForNextTickDone)
	}()

	// Shutdown the engine at some point in the near future
	time.AfterFunc(
		100*time.Millisecond, func() {
			engine.Shutdown()
		},
	)
	// testTimeout will cause the test to fail if we have to wait too long for a WaitForNextTick failure
	testTimeout := time.After(5 * time.Second)
	for {
		select {
		case startTickCh <- time.Now():
			time.Sleep(10 * time.Millisecond)
			<-doneTickCh
		case <-testTimeout:
			assert.Check(t, false, "test timeout")
			return
		case <-waitForNextTickDone:
			// WaitForNextTick failed, meaning this test was successful
			return
		}
	}
}

func TestCannotWaitForNextTickAfterEngineIsShutDown(t *testing.T) {
	engine := testutils.NewTestFixture(t, nil).Engine
	startTickCh := make(chan time.Time)
	doneTickCh := make(chan uint64)
	assert.NilError(t, engine.LoadGameState())
	engine.StartGameLoop(context.Background(), startTickCh, doneTickCh)

	// Make sure the game can tick
	startTickCh <- time.Now()
	<-doneTickCh

	engine.Shutdown()

	for i := 0; i < 10; i++ {
		// After a engine is shut down, WaitForNextTick should never block and always fail
		assert.Check(t, !engine.WaitForNextTick())
	}
}

func TestEVMTxConsume(t *testing.T) {
	ctx := context.Background()
	type FooIn struct {
		X uint32
	}
	type FooOut struct {
		Y string
	}
	e := testutils.NewTestFixture(t, nil).Engine
	fooTx := ecs.NewMessageType[FooIn, FooOut]("foo", ecs.WithMsgEVMSupport[FooIn, FooOut])
	assert.NilError(t, e.RegisterMessages(fooTx))
	var returnVal FooOut
	var returnErr error
	e.RegisterSystem(
		func(eCtx ecs.EngineContext) error {
			fooTx.Each(
				eCtx, func(t ecs.TxData[FooIn]) (FooOut, error) {
					return returnVal, returnErr
				},
			)
			return nil
		},
	)
	assert.NilError(t, e.LoadGameState())

	// add tx to queue
	evmTxHash := "0xFooBar"
	e.AddEVMTransaction(fooTx.ID(), FooIn{X: 32}, &sign.Transaction{PersonaTag: "foo"}, evmTxHash)

	// let's check against a system that returns a result and no error
	returnVal = FooOut{Y: "hi"}
	returnErr = nil
	assert.NilError(t, e.Tick(ctx))
	evmTxReceipt, ok := e.ConsumeEVMMsgResult(evmTxHash)
	assert.Equal(t, ok, true)
	assert.Check(t, len(evmTxReceipt.ABIResult) > 0)
	assert.Equal(t, evmTxReceipt.EVMTxHash, evmTxHash)
	assert.Equal(t, len(evmTxReceipt.Errs), 0)
	// shouldn't be able to consume it again.
	_, ok = e.ConsumeEVMMsgResult(evmTxHash)
	assert.Equal(t, ok, false)

	// lets check against a system that returns an error
	returnVal = FooOut{}
	returnErr = errors.New("omg error")
	e.AddEVMTransaction(fooTx.ID(), FooIn{X: 32}, &sign.Transaction{PersonaTag: "foo"}, evmTxHash)
	assert.NilError(t, e.Tick(ctx))
	evmTxReceipt, ok = e.ConsumeEVMMsgResult(evmTxHash)

	assert.Equal(t, ok, true)
	assert.Equal(t, len(evmTxReceipt.ABIResult), 0)
	assert.Equal(t, evmTxReceipt.EVMTxHash, evmTxHash)
	assert.Equal(t, len(evmTxReceipt.Errs), 1)
	// shouldn't be able to consume it again.
	_, ok = e.ConsumeEVMMsgResult(evmTxHash)
	assert.Equal(t, ok, false)
}

func TestAddSystems(t *testing.T) {
	count := 0
	sys := func(ecs.EngineContext) error {
		count++
		return nil
	}

	engine := testutils.NewTestFixture(t, nil).Engine
	engine.RegisterSystems(sys, sys, sys)
	err := engine.LoadGameState()
	assert.NilError(t, err)

	err = engine.Tick(context.Background())
	assert.NilError(t, err)

	assert.Equal(t, count, 3)
}

func TestSystemExecutionOrder(t *testing.T) {
	engine := testutils.NewTestFixture(t, nil).Engine
	order := make([]int, 0, 3)
	engine.RegisterSystems(
		func(ecs.EngineContext) error {
			order = append(order, 1)
			return nil
		}, func(ecs.EngineContext) error {
			order = append(order, 2)
			return nil
		}, func(ecs.EngineContext) error {
			order = append(order, 3)
			return nil
		},
	)
	err := engine.LoadGameState()
	assert.NilError(t, err)
	assert.NilError(t, engine.Tick(context.Background()))
	expectedOrder := []int{1, 2, 3}
	for i, elem := range order {
		assert.Equal(t, elem, expectedOrder[i])
	}
}

func TestSetNamespace(t *testing.T) {
	namespace := "test"
	t.Setenv("CARDINAL_NAMESPACE", namespace)
	e := testutils.NewTestFixture(t, nil).Engine
	assert.Equal(t, e.Namespace().String(), namespace)
}

func TestWithoutRegistration(t *testing.T) {
	engine := testutils.NewTestFixture(t, nil).Engine
	eCtx := ecs.NewEngineContext(engine)
	id, err := ecs.Create(eCtx, EnergyComponent{}, OwnableComponent{})
	assert.Assert(t, err != nil)

	err = ecs.UpdateComponent[EnergyComponent](
		eCtx, id, func(component *EnergyComponent) *EnergyComponent {
			component.Amt += 50
			return component
		},
	)
	assert.Assert(t, err != nil)

	err = ecs.SetComponent[EnergyComponent](
		eCtx, id, &EnergyComponent{
			Amt: 0,
			Cap: 0,
		},
	)

	assert.Assert(t, err != nil)

	assert.NilError(t, ecs.RegisterComponent[EnergyComponent](engine))
	assert.NilError(t, ecs.RegisterComponent[OwnableComponent](engine))
	id, err = ecs.Create(eCtx, EnergyComponent{}, OwnableComponent{})
	assert.NilError(t, err)
	err = ecs.UpdateComponent[EnergyComponent](
		eCtx, id, func(component *EnergyComponent) *EnergyComponent {
			component.Amt += 50
			return component
		},
	)
	assert.NilError(t, err)
	err = ecs.SetComponent[EnergyComponent](
		eCtx, id, &EnergyComponent{
			Amt: 0,
			Cap: 0,
		},
	)
	assert.NilError(t, err)
}

type dummyAdapter struct {
	txs           txpool.TxMap
	ns            string
	epoch         uint64
	unixTimestamp uint64
}

func (d *dummyAdapter) Submit(
	_ context.Context,
	txs txpool.TxMap,
	namespace string,
	epoch, unixTimestamp uint64,
) error {
	d.txs = txs
	d.ns = namespace
	d.epoch = epoch
	d.unixTimestamp = unixTimestamp
	return nil
}

func (d *dummyAdapter) QueryTransactions(_ context.Context, _ *types.QueryTransactionsRequest) (
	*types.QueryTransactionsResponse, error,
) {
	return &types.QueryTransactionsResponse{}, nil
}

var _ adapter.Adapter = &dummyAdapter{}

// TestAdapterCalledAfterTick tests that when messages are executed in a tick, they are forwarded to the adapter.
func TestAdapterCalledAfterTick(t *testing.T) {
	adapter := &dummyAdapter{}
	engine := testutils.NewTestFixture(t, nil, cardinal.WithAdapter(adapter)).Engine

	engine.RegisterSystem(func(engineContext ecs.EngineContext) error {
		return nil
	})
	fooMessage := ecs.NewMessageType[struct{}, struct{}]("foo")
	err := engine.RegisterMessages(fooMessage)
	assert.NilError(t, err)
	err = engine.LoadGameState()
	assert.NilError(t, err)

	fooMessage.AddToQueue(engine, struct{}{}, &sign.Transaction{
		PersonaTag: "meow",
		Namespace:  "foo",
		Nonce:      22,
		Signature:  "meow",
		Hash:       common.Hash{},
		Body:       json.RawMessage(`{}`),
	})
	fooMessage.AddToQueue(engine, struct{}{}, &sign.Transaction{
		PersonaTag: "meow",
		Namespace:  "foo",
		Nonce:      23,
		Signature:  "meow",
		Hash:       common.Hash{},
		Body:       json.RawMessage(`{}`),
	})
	err = engine.Tick(context.Background())
	assert.NilError(t, err)

	assert.Len(t, adapter.txs[fooMessage.ID()], 2)
	assert.Equal(t, engine.Namespace().String(), adapter.ns)
	assert.Equal(t, engine.CurrentTick()-1, adapter.epoch)
}