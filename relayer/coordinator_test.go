package relayer_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fury-labs/fury-bridge/relayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoordinator_EthToFurySigning tests that the coordinator produces the correct
// signing outputs for all permuations of block sequences.  This ensures that relayers
// receiving blocks in any order in real time or during syncing always produce the
// same signing outputs.
//
// There are (len(eth) + len(fury)) choose len(eth) possible permunations, so it's
// important to keep the number of blocks in each example low.
func TestCoordinator_EthToFurySigning(t *testing.T) {
	refTime := time.Now()
	confTime := refTime.Add(-5 * time.Minute)

	testCases := []struct {
		name            string
		ethblocks       []relayer.Block
		furyblocks      []relayer.Block
		expectedOutputs [][]expectedOutput
	}{
		{
			"no payloads",
			blocks(
				eth(1, addSecond(confTime, 0)),
				eth(2, addSecond(confTime, 1)),
				eth(3, addSecond(confTime, 2)),
				eth(4, addSecond(confTime, 3)),
				eth(5, addSecond(confTime, 4)),
			),
			blocks(
				fury(1, addSecond(refTime, 0)),
			),
			outputs(),
		},
		{
			"assorted payloads with no retry",
			blocks(
				eth(1, addSecond(confTime, 0)),
				eth(2, addSecond(confTime, 1), newPayload(1)),
				eth(3, addSecond(confTime, 2), newPayload(2), newPayload(3)),
				eth(4, addSecond(confTime, 3)),
				eth(5, addSecond(confTime, 4), newPayload(4), newPayload(5), newPayload(6)),
				eth(6, addSecond(confTime, 5)),
			),
			blocks(
				fury(1, addSecond(refTime, 0)),
				fury(2, addSecond(refTime, 1)),
				fury(3, addSecond(refTime, 2)),
				fury(4, addSecond(refTime, 3)),
				fury(5, addSecond(refTime, 4)),
				fury(6, addSecond(refTime, 5)),
			),
			outputs(
				output(
					newOutput(1, newPayload(1)),
				),
				output(
					newOutput(2, newPayload(2)),
					newOutput(3, newPayload(3)),
				),
				output(
					newOutput(4, newPayload(4)),
					newOutput(5, newPayload(5)),
					newOutput(6, newPayload(6)),
				),
			),
		},
		{
			"simple retry",
			blocks(
				eth(1, addSecond(confTime, 0)),
				eth(2, addSecond(confTime, 15), newPayload(10)),
				eth(3, addSecond(confTime, 30)),
				eth(4, addSecond(refTime, 2)),
			),
			blocks(
				fury(1, addSecond(refTime, 0)),
				fury(2, addSecond(refTime, 1), newPayload(10)),
				fury(3, addSecond(refTime, 3)),
			),
			outputs(
				output(
					newOutput(1, newPayload(10)),
				),
				output(
					newOutput(2, newPayload(10)),
				),
			),
		},
		{
			"retry with equal timestamp",
			blocks(
				eth(1, addSecond(confTime, 0)),
				eth(2, addSecond(confTime, 15), newPayload(1)),
				eth(3, addSecond(confTime, 30)),
				eth(4, addSecond(refTime, 1), newPayload(2)),
				eth(5, addSecond(refTime, 2)),
			),
			blocks(
				fury(1, addSecond(refTime, 0)),
				fury(2, addSecond(refTime, 1), newPayload(1)),
				fury(3, addSecond(refTime, 3)),
			),
			outputs(
				output(
					newOutput(1, newPayload(1)),
				),
				output(
					newOutput(2, newPayload(2)),
				),
				output(
					newOutput(3, newPayload(1)),
				),
			),
		},
		{
			"complex retry",
			blocks(
				eth(1, addSecond(refTime, 0)),
				eth(2, addSecond(refTime, 10), newPayload(2)),
				eth(3, addSecond(refTime, 20), newPayload(3), newPayload(4)),
				eth(4, addSecond(refTime, 30)),
				eth(5, addSecond(refTime, 40), newPayload(5), newPayload(6), newPayload(7)),
				eth(6, addSecond(refTime, 50)),
			),
			blocks(
				fury(1, addSecond(refTime, 1)),
				fury(2, addSecond(refTime, 9), newPayload(1)),
				fury(3, addSecond(refTime, 19)),
				fury(4, addSecond(refTime, 29), newPayload(3), newPayload(4)),
				fury(5, addSecond(refTime, 39), newPayload(5)),
				fury(6, addSecond(refTime, 49), newPayload(6)),
				fury(7, addSecond(refTime, 59)),
			),
			outputs(
				output(
					newOutput(1, newPayload(2)),
					newOutput(2, newPayload(1)),
				),
				output(
					newOutput(3, newPayload(3)),
					newOutput(4, newPayload(4)),
				),
				output(
					newOutput(5, newPayload(3)),
					newOutput(6, newPayload(4)),
				),
				output(
					newOutput(7, newPayload(5)),
					newOutput(8, newPayload(6)),
					newOutput(9, newPayload(7)),
					newOutput(10, newPayload(5)),
				),
				output(
					newOutput(11, newPayload(6)),
				),
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			permutations := [][]relayer.Block{}
			orderedBlockPermutations(&permutations, []relayer.Block{}, tc.ethblocks, tc.furyblocks)

			for index, permutation := range permutations {
				coordinator := newCoordinator()

				outputs, err := writeBlocks(ctx, coordinator, permutation)
				require.NoError(t, err)

				var actualOutputs [][]expectedOutput
				for _, output := range outputs {
					actualOut := []expectedOutput{}
					for _, o := range output {
						actualOut = append(actualOut, expectedOutput{o.Nonce(), o.Payload()})
					}
					actualOutputs = append(actualOutputs, actualOut)
				}

				errDetail := fmt.Sprintf("permutation %d of %d failed", index+1, len(permutations))
				require.Equal(t, tc.expectedOutputs, actualOutputs, errDetail)
			}
		})
	}
}

// TestCoordinator_InvalidSequences ensures blocks are never processed in a sequence
// that would result in non-deterministic actions on the destination (fury) chain.
//
// This ensures the coordinator is protected from faults in block processing and provides
// enough error information for a block processor to recover.  In addition, we can be
// confident that the core logic in the coordinator is not corrupted by an invalid
// sequence of new information.
//
// TODO: we can improve the error messages to be more specific?
func TestCoordinator_InvalidSequences(t *testing.T) {
	refTime := time.Now()

	testCases := []struct {
		name        string
		initBlocks  []relayer.Block
		block       relayer.Block
		expectedErr error
	}{
		// A block with the same height can not be added
		{"fury same height", blocks(fury(1, refTime)), fury(1, addSecond(refTime, 1)), relayer.ErrInvalidBlockHeight},
		{"eth same height", blocks(fury(1, refTime), eth(1, addSecond(refTime, -2))), eth(1, addSecond(refTime, -1)), relayer.ErrInvalidBlockHeight},
		// A block with a previous height can not be added
		{"fury previous height", blocks(fury(1, refTime), fury(2, addSecond(refTime, 1))), fury(2, addSecond(refTime, 2)), relayer.ErrInvalidBlockHeight},
		{"eth previous height", blocks(fury(1, refTime), eth(1, addSecond(refTime, -3)), eth(2, addSecond(refTime, -2))), eth(1, addSecond(refTime, -1)), relayer.ErrInvalidBlockHeight},
		// A block with a skipped height can not be added
		{"fury previous height", blocks(fury(1, refTime)), fury(3, addSecond(refTime, 1)), relayer.ErrInvalidBlockHeight},
		{"eth previous height", blocks(fury(1, refTime), eth(1, addSecond(refTime, -2))), eth(3, addSecond(refTime, -1)), relayer.ErrInvalidBlockHeight},
		// A block with the same timestamp can not be added
		{"fury previous height", blocks(fury(1, refTime)), fury(2, addSecond(refTime, 0)), relayer.ErrInvalidBlockTime},
		{"eth previous height", blocks(fury(1, refTime), eth(1, addSecond(refTime, -1))), eth(2, addSecond(refTime, -1)), relayer.ErrInvalidBlockTime},
		// A block with a previous timestamp can not be added
		{"fury previous height", blocks(fury(1, refTime)), fury(2, addSecond(refTime, -1)), relayer.ErrInvalidBlockTime},
		{"eth previous height", blocks(fury(1, refTime), eth(1, addSecond(refTime, -1))), eth(2, addSecond(refTime, -2)), relayer.ErrInvalidBlockTime},

		// An eth block with a timestamp greater than the last fury block can not be added
		{"first eth timestsamp greater than fury", blocks(fury(1, refTime)), eth(1, addSecond(refTime, 1)), relayer.ErrSourceBlockAhead},
		{"next eth timestsamp greater than fury", blocks(fury(1, refTime), eth(1, addSecond(refTime, -1))), eth(2, addSecond(refTime, 1)), relayer.ErrSourceBlockAhead},
		// An eth block with a timestamp equal than the last fury block can not be added
		{"eth timestsamp equal to fury", blocks(fury(1, refTime)), eth(1, addSecond(refTime, 0)), relayer.ErrSourceBlockAhead},
		{"next eth timestsamp equal than fury", blocks(fury(1, refTime), eth(1, addSecond(refTime, -1))), eth(2, addSecond(refTime, 0)), relayer.ErrSourceBlockAhead},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coordinator := newCoordinator()
			stop := startNullReader(coordinator)
			defer stop()

			// initialize coordinator with some state
			for _, blk := range tc.initBlocks {
				err := coordinator.AddBlock(context.Background(), blk)
				require.NoError(t, err, "expected no errors when setting initial state")
			}

			// test block addition and assert error
			err := coordinator.AddBlock(context.Background(), tc.block)
			assert.Equal(t, tc.expectedErr, err, "expected an error to be returned")
		})
	}
}

// TestCoordinator_AllowedSequences tests that normal operation sequences still succeed.
// This ensures assumptions made in valid block processing are not violated.
func TestCoordinator_AllowedSequences(t *testing.T) {
	refTime := time.Now()

	testCases := []struct {
		name   string
		blocks []relayer.Block
	}{
		// a single fury block to initialize the coordniator is always allowed
		{
			"single fury block",
			blocks(
				fury(1, refTime),
			),
		},
		// fury blocks never cause actions, so any number can be added with disregard for eth blocks
		{
			"fury blocks only",
			blocks(
				fury(1, refTime),
				fury(2, addSecond(refTime, 1)),
				fury(3, addSecond(refTime, 2)),
				fury(4, addSecond(refTime, 3)),
				fury(5, addSecond(refTime, 4)),
			),
		},
		// as long no validations are violated, we may alternate between fury and eth blocks
		{
			"alternating blocks",
			blocks(
				fury(1, refTime),
				eth(1, addSecond(refTime, -1)),
				fury(2, addSecond(refTime, 1)),
				eth(2, addSecond(refTime, 0)),
				fury(3, addSecond(refTime, 2)),
				eth(3, addSecond(refTime, 1)),
			),
		},
		// any length sequence of eth blocks as long the timestamps are less than the last added fury block
		{
			"sequence of eth blocks",
			blocks(
				fury(1, refTime),
				eth(1, addSecond(refTime, -5)),
				eth(2, addSecond(refTime, -4)),
				eth(3, addSecond(refTime, -3)),
				eth(4, addSecond(refTime, -2)),
				eth(5, addSecond(refTime, -1)),
			),
		},
		// any alternating sequences of block runs is allowed as long as validations are not violated
		{
			"alternating sequences",
			blocks(
				fury(1, addSecond(refTime, -4)),
				fury(2, addSecond(refTime, -3)),
				fury(3, addSecond(refTime, -2)),
				fury(4, addSecond(refTime, -1)),
				fury(5, addSecond(refTime, 0)),
				eth(1, addSecond(refTime, -5)),
				eth(2, addSecond(refTime, -4)),
				eth(3, addSecond(refTime, -3)),
				eth(4, addSecond(refTime, -2)),
				eth(5, addSecond(refTime, -1)),
				fury(6, addSecond(refTime, 1)),
				fury(7, addSecond(refTime, 2)),
				fury(8, addSecond(refTime, 3)),
				fury(9, addSecond(refTime, 4)),
				eth(6, addSecond(refTime, 0)),
				eth(7, addSecond(refTime, 1)),
				eth(8, addSecond(refTime, 2)),
				eth(9, addSecond(refTime, 3)),
			),
		},
		// no validations are violated with overlapping timestamp ranges
		{
			"overlapping timestamp ranges",
			blocks(
				fury(1, addSecond(refTime, -5)),
				fury(2, addSecond(refTime, 0)),
				eth(1, addSecond(refTime, -5)),
				eth(2, addSecond(refTime, -4)),
				eth(3, addSecond(refTime, -3)),
				eth(4, addSecond(refTime, -2)),
				eth(5, addSecond(refTime, -1)),
				fury(3, addSecond(refTime, 10)),
				eth(6, addSecond(refTime, 0)),
				eth(7, addSecond(refTime, 5)),
				eth(8, addSecond(refTime, 8)),
				eth(9, addSecond(refTime, 9)),
				fury(4, addSecond(refTime, 11)),
			),
		},
		{
			"large timestamp gaps",
			blocks(
				fury(1, addSecond(refTime, -1000)),
				fury(2, addSecond(refTime, 0)),
				eth(1, addSecond(refTime, -999)),
				eth(2, addSecond(refTime, -1)),
				fury(3, addSecond(refTime, 1000)),
				eth(3, addSecond(refTime, 500)),
				fury(4, addSecond(refTime, 1001)),
				eth(4, addSecond(refTime, 1000)),
			),
		},
		// blocks may start processing at any block height, not just height 1
		// TODO: this requires special initialization
		//{
		//	"start at any block height",
		//	blocks(
		//		fury(100, addSecond(refTime, 0)),
		//		eth(100, addSecond(refTime, -1)),
		//	),
		//},
		// blocks may start processing at a future time (processing is not dependent on real clock)
		{
			"timestamps may start any an future time",
			blocks(
				fury(1, addSecond(refTime, 300)),
				eth(1, addSecond(refTime, 299)),
			),
		},
		{
			"timestamps may start any past time",
			blocks(
				fury(1, addSecond(refTime, -299)),
				eth(1, addSecond(refTime, -300)),
			),
		},
		{
			"multiple retries of a payload",
			blocks(
				fury(1, addSecond(refTime, 0)),
				eth(1, addSecond(refTime, -15), newPayload(1)),
				fury(2, addSecond(refTime, 6), newPayload(1)),
				fury(3, addSecond(refTime, 12), newPayload(1)),
				eth(2, addSecond(refTime, 0)),
				fury(4, addSecond(refTime, 18), newPayload(1)),
				eth(3, addSecond(refTime, 15), newPayload(2)),
				fury(5, addSecond(refTime, 24), newPayload(2)),
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coordinator := newCoordinator()
			stop := startNullReader(coordinator)
			defer stop()

			// initialize coordinator with some state
			for _, blk := range tc.blocks {
				err := coordinator.AddBlock(context.Background(), blk)
				require.NoError(t, err, "expected no errors when processing blocks")
			}
		})
	}
}

// TestCoordinator_AddBlockCancellation tests that AddBlock does not process any outputs
// or modify state on context cancellation
func TestCoordinator_AddBlockCancellation(t *testing.T) {
	refTime := time.Now()
	coordinator := newCoordinator()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	err := coordinator.AddBlock(ctx, fury(1, refTime))
	require.NoError(t, err)

	cancel()

	err = coordinator.AddBlock(ctx, eth(1, addSecond(refTime, -1), newPayload(1)))
	require.EqualError(t, err, "context canceled")

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	stop := startNullReader(coordinator)
	defer stop()

	err = coordinator.AddBlock(ctx, eth(1, addSecond(refTime, -1), newPayload(1)))
	require.NoError(t, err)
}

// TestCoordinator_UnknownBlockType ensures AddBlock throws an error if unrecognized block is added
func TestCoordinator_UnknownBlockType(t *testing.T) {
	coordinator := newCoordinator()

	err := coordinator.AddBlock(context.Background(), relayer.NewBlock(100, 1, time.Now(), nil))

	assert.Equal(t, relayer.ErrUnknownBlockOrigin, err, "expected a unkown block origin to error")
}
