package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	keepertest "github.com/zeta-chain/zetacore/testutil/keeper"
	"github.com/zeta-chain/zetacore/testutil/sample"
	"github.com/zeta-chain/zetacore/x/crosschain/keeper"
	"github.com/zeta-chain/zetacore/x/crosschain/types"
	observertypes "github.com/zeta-chain/zetacore/x/observer/types"
)

// createCctxWithNonceRange create in the store:
// mined cctx from nonce 0 to low
// pending cctx from low to high
// set pending nonces from low to higg
// return pending cctxs
func createCctxWithNonceRange(
	t *testing.T,
	ctx sdk.Context,
	k keeper.Keeper,
	lowPending int,
	highPending int,
	chainID int64,
	tss observertypes.TSS,
	zk keepertest.ZetaKeepers,
) (cctxs []*types.CrossChainTx) {
	for i := 0; i < lowPending; i++ {
		cctx := sample.CrossChainTx(t, fmt.Sprintf("%d-%d", chainID, i))
		cctx.CctxStatus.Status = types.CctxStatus_OutboundMined
		cctx.InboundTxParams.SenderChainId = chainID
		k.SetCrossChainTx(ctx, *cctx)
		zk.ObserverKeeper.SetNonceToCctx(ctx, observertypes.NonceToCctx{
			ChainId:   chainID,
			Nonce:     int64(i),
			CctxIndex: cctx.Index,
			Tss:       tss.TssPubkey,
		})
	}
	for i := lowPending; i < highPending; i++ {
		cctx := sample.CrossChainTx(t, fmt.Sprintf("%d-%d", chainID, i))
		cctx.CctxStatus.Status = types.CctxStatus_PendingOutbound
		cctx.InboundTxParams.SenderChainId = chainID
		k.SetCrossChainTx(ctx, *cctx)
		zk.ObserverKeeper.SetNonceToCctx(ctx, observertypes.NonceToCctx{
			ChainId:   chainID,
			Nonce:     int64(i),
			CctxIndex: cctx.Index,
			Tss:       tss.TssPubkey,
		})
		cctxs = append(cctxs, cctx)
	}
	zk.ObserverKeeper.SetPendingNonces(ctx, observertypes.PendingNonces{
		ChainId:   chainID,
		NonceLow:  int64(lowPending),
		NonceHigh: int64(highPending),
		Tss:       tss.TssPubkey,
	})

	return
}

func TestKeeper_CctxListPending(t *testing.T) {

	t.Run("should fail for empty req", func(t *testing.T) {
		k, ctx, _, _ := keepertest.CrosschainKeeper(t)
		_, err := k.CctxListPending(ctx, nil)
		require.ErrorContains(t, err, "invalid request")
	})

	t.Run("should fail if limit is too high", func(t *testing.T) {
		k, ctx, _, _ := keepertest.CrosschainKeeper(t)
		_, err := k.CctxListPending(ctx, &types.QueryListCctxPendingRequest{Limit: keeper.MaxPendingCctxs + 1})
		require.ErrorContains(t, err, "limit exceeds max limit of")
	})

	t.Run("should fail if no TSS", func(t *testing.T) {
		k, ctx, _, _ := keepertest.CrosschainKeeper(t)
		_, err := k.CctxListPending(ctx, &types.QueryListCctxPendingRequest{Limit: 1})
		require.ErrorContains(t, err, "tss not found")
	})

	t.Run("should return empty list if no nonces", func(t *testing.T) {
		k, ctx, _, zk := keepertest.CrosschainKeeper(t)

		//  set TSS
		zk.ObserverKeeper.SetTSS(ctx, sample.Tss())

		_, err := k.CctxListPending(ctx, &types.QueryListCctxPendingRequest{Limit: 1})
		require.ErrorContains(t, err, "pending nonces not found")
	})

	t.Run("can retrieve pending cctx in range", func(t *testing.T) {
		k, ctx, _, zk := keepertest.CrosschainKeeper(t)
		chainID := getValidEthChainID(t)
		tss := sample.Tss()
		zk.ObserverKeeper.SetTSS(ctx, tss)
		cctxs := createCctxWithNonceRange(t, ctx, *k, 1000, 2000, chainID, tss, zk)

		res, err := k.CctxListPending(ctx, &types.QueryListCctxPendingRequest{ChainId: chainID, Limit: 100})
		require.NoError(t, err)
		require.Equal(t, 100, len(res.CrossChainTx))
		require.EqualValues(t, cctxs[0:100], res.CrossChainTx)
		require.EqualValues(t, uint64(1000), res.TotalPending)

		res, err = k.CctxListPending(ctx, &types.QueryListCctxPendingRequest{ChainId: chainID})
		require.NoError(t, err)
		require.Equal(t, keeper.MaxPendingCctxs, len(res.CrossChainTx))
		require.EqualValues(t, cctxs[0:keeper.MaxPendingCctxs], res.CrossChainTx)
		require.EqualValues(t, uint64(1000), res.TotalPending)
	})

	t.Run("can retrieve pending cctx with range smaller than max", func(t *testing.T) {
		k, ctx, _, zk := keepertest.CrosschainKeeper(t)
		chainID := getValidEthChainID(t)
		tss := sample.Tss()
		zk.ObserverKeeper.SetTSS(ctx, tss)
		cctxs := createCctxWithNonceRange(t, ctx, *k, 1000, 1100, chainID, tss, zk)

		res, err := k.CctxListPending(ctx, &types.QueryListCctxPendingRequest{ChainId: chainID})
		require.NoError(t, err)
		require.Equal(t, 100, len(res.CrossChainTx))
		require.EqualValues(t, cctxs, res.CrossChainTx)
		require.EqualValues(t, uint64(100), res.TotalPending)
	})

	t.Run("can retrieve pending cctx with pending cctx below nonce low", func(t *testing.T) {
		k, ctx, _, zk := keepertest.CrosschainKeeper(t)
		chainID := getValidEthChainID(t)
		tss := sample.Tss()
		zk.ObserverKeeper.SetTSS(ctx, tss)
		cctxs := createCctxWithNonceRange(t, ctx, *k, 1000, 2000, chainID, tss, zk)

		// set some cctxs as pending below nonce
		cctx1, found := k.GetCrossChainTx(ctx, "1337-940")
		require.True(t, found)
		cctx1.CctxStatus.Status = types.CctxStatus_PendingOutbound
		k.SetCrossChainTx(ctx, cctx1)

		cctx2, found := k.GetCrossChainTx(ctx, "1337-955")
		require.True(t, found)
		cctx2.CctxStatus.Status = types.CctxStatus_PendingOutbound
		k.SetCrossChainTx(ctx, cctx2)

		res, err := k.CctxListPending(ctx, &types.QueryListCctxPendingRequest{ChainId: chainID, Limit: 100})
		require.NoError(t, err)
		require.Equal(t, 100, len(res.CrossChainTx))

		expectedCctxs := append([]*types.CrossChainTx{&cctx1, &cctx2}, cctxs[0:98]...)
		require.EqualValues(t, expectedCctxs, res.CrossChainTx)

		// pending nonce + 2
		require.EqualValues(t, uint64(1002), res.TotalPending)
	})
}
