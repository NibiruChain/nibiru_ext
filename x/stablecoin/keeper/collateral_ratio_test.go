package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
	"github.com/NibiruChain/nibiru/x/common/testutil/testapp"
	"github.com/NibiruChain/nibiru/x/stablecoin/types"
)

func TestSetCollRatio_Input(t *testing.T) {
	type TestCase struct {
		name         string
		inCollRatio  sdk.Dec
		expectedPass bool
	}

	executeTest := func(t *testing.T, testCase TestCase) {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper

			err := stablecoinKeeper.SetCollRatio(ctx, tc.inCollRatio)
			if tc.expectedPass {
				require.NoError(
					t, err, "Error setting the CollRatio: %d", tc.inCollRatio)
				return
			}
			require.Error(t, err)
		})
	}

	testCases := []TestCase{
		{
			name:         "Upper bound of CollRatio",
			inCollRatio:  sdk.OneDec(),
			expectedPass: true,
		}, {
			name:         "Lower bound of CollRatio",
			inCollRatio:  sdk.ZeroDec(),
			expectedPass: true,
		}, {
			name:         "CollRatio above 100",
			inCollRatio:  sdk.MustNewDecFromStr("1.5"),
			expectedPass: false,
		}, {
			name:         "Negative CollRatio not allowed",
			inCollRatio:  sdk.OneDec().Neg(),
			expectedPass: false,
		},
	}
	for _, testCase := range testCases {
		executeTest(t, testCase)
	}
}

func TestSetCollRatioUpdate(t *testing.T) {
	type TestCase struct {
		name              string
		inCollRatio       sdk.Dec
		price             sdk.Dec
		expectedCollRatio sdk.Dec
		expectedPass      bool
	}

	executeTest := func(t *testing.T, testCase TestCase) {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)

			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			oracleKeeper := &nibiruApp.OracleKeeper

			oracleKeeper.SetPrice(ctx, asset.Registry.Pair(denoms.USDC, denoms.NUSD), tc.price)
			err := stablecoinKeeper.SetCollRatio(ctx, tc.inCollRatio)
			require.NoError(t, err)
			err = stablecoinKeeper.EvaluateCollRatio(ctx)
			if tc.expectedPass {
				require.NoError(
					t, err, "Error setting the CollRatio: %d", tc.inCollRatio)

				currCollRatio := stablecoinKeeper.GetCollRatio(ctx)
				require.Equal(t, tc.expectedCollRatio, currCollRatio)
				return
			}
			require.Error(t, err)
		})
	}

	testCases := []TestCase{
		{
			name:              "Collateral price is higher than stable",
			inCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("1.1"),
			expectedCollRatio: sdk.MustNewDecFromStr("0.8025"),
			expectedPass:      true,
		},
		{
			name:              "Price is slightly higher than peg",
			inCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("1.00000001"),
			expectedCollRatio: sdk.MustNewDecFromStr("0.8"),
			expectedPass:      true,
		},
		{
			name:              "Price is slightly lower than peg",
			inCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("0.99999999991"),
			expectedCollRatio: sdk.MustNewDecFromStr("0.8"),
			expectedPass:      true,
		},
		{
			name:              "Collateral price is lower than stable",
			inCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("0.9"),
			expectedCollRatio: sdk.MustNewDecFromStr("0.7975"),
			expectedPass:      true,
		},
	}
	for _, testCase := range testCases {
		executeTest(t, testCase)
	}
}

func TestGetCollRatio_Input(t *testing.T) {
	testName := "GetCollRatio after setting default params returns expected value"
	t.Run(testName, func(t *testing.T) {
		nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
		stablecoinKeeper := &nibiruApp.StablecoinKeeper

		stablecoinKeeper.SetParams(ctx, types.DefaultParams())
		expectedCollRatioInt := sdk.NewInt(types.DefaultParams().CollRatio)

		outCollRatio := stablecoinKeeper.GetCollRatio(ctx)
		outCollRatioInt := outCollRatio.Mul(sdk.MustNewDecFromStr("1000000")).RoundInt()
		require.EqualValues(t, expectedCollRatioInt, outCollRatioInt)
	})

	testName = "Setting to non-default value returns expected value"
	t.Run(testName, func(t *testing.T) {
		nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
		stablecoinKeeper := &nibiruApp.StablecoinKeeper

		expectedCollRatio := sdk.MustNewDecFromStr("0.5")
		expectedCollRatioInt := expectedCollRatio.Mul(sdk.MustNewDecFromStr("1000000")).RoundInt()
		require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, expectedCollRatio))

		outCollRatio := stablecoinKeeper.GetCollRatio(ctx)
		outCollRatioInt := outCollRatio.Mul(sdk.MustNewDecFromStr("1000000")).RoundInt()
		require.EqualValues(t, expectedCollRatioInt, outCollRatioInt)
	})
}

func TestStableRequiredForTargetCollRatio(t *testing.T) {
	testCases := []struct {
		name             string
		protocolColl     sdk.Int
		priceCollStable  sdk.Dec
		postedAssetPairs []asset.Pair
		stableSupply     sdk.Int
		targetCollRatio  sdk.Dec
		neededUSD        sdk.Dec

		expectedPass bool
	}{
		{
			name:            "Too little collateral gives correct positive value",
			protocolColl:    sdk.NewInt(500),
			priceCollStable: sdk.OneDec(), // startCollUSD = 500 * 1 -> 500
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.6"), // 0.6 * 1000 = 600
			neededUSD:       sdk.MustNewDecFromStr("100"), // = 600 - 500
			expectedPass:    true,
		}, {
			name:            "Too much collateral gives correct negative value",
			protocolColl:    sdk.NewInt(600),
			priceCollStable: sdk.OneDec(), // startCollUSD = 600 * 1 = 600
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.5"),  // 0.5 * 1000 = 500
			neededUSD:       sdk.MustNewDecFromStr("-100"), // = 500 - 600
			expectedPass:    true,
		}, {
			name:             "No price available for the collateral",
			protocolColl:     sdk.NewInt(500),
			priceCollStable:  sdk.OneDec(), // startCollUSD = 500 * 1 -> 500
			postedAssetPairs: []asset.Pair{},
			stableSupply:     sdk.NewInt(1_000),
			targetCollRatio:  sdk.MustNewDecFromStr("0.6"), // 0.6 * 1000 = 600
			neededUSD:        sdk.MustNewDecFromStr("100"), // = 600 - 500
			expectedPass:     false,
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, tc.targetCollRatio))
			require.NoError(t, nibiruApp.BankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(
					sdk.NewCoin(denoms.USDC, tc.protocolColl),
					sdk.NewCoin(denoms.NUSD, tc.stableSupply),
				),
			))

			// Post prices to each specified market with the oracle.
			prices := map[asset.Pair]sdk.Dec{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD): tc.priceCollStable,
			}
			for _, pair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, pair, prices[pair])
			}

			neededUSD, err := stablecoinKeeper.StableRequiredForTargetCollRatio(ctx)
			if tc.expectedPass {
				require.NoError(t, err)
				require.EqualValues(t, tc.neededUSD, neededUSD)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestRecollateralizeCollAmtForTargetCollRatio(t *testing.T) {
	type TestCaseRecollateralizeCollAmtForTargetCollRatio struct {
		name            string
		protocolColl    sdk.Int
		priceCollStable sdk.Dec
		stableSupply    sdk.Int
		targetCollRatio sdk.Dec
		neededCollAmt   sdk.Int
		expectedPass    bool
	}

	expectedPasses := []TestCaseRecollateralizeCollAmtForTargetCollRatio{
		{
			name:            "under-collateralized; untruncated integer amount",
			protocolColl:    sdk.NewInt(500),
			priceCollStable: sdk.OneDec(), // startCollUSD = 500 * 1 -> 500
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.6"), // 0.6 * 1000 = 600
			neededCollAmt:   sdk.NewInt(100),              // = 600 - 500
			expectedPass:    true,
		},
		{
			name:            "under-collateralized; truncated integer amount",
			protocolColl:    sdk.NewInt(500),
			priceCollStable: sdk.OneDec(), // startCollUSD = 500 * 1 -> 500
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.6025"), // 0.6025 * 1000 = 602.5
			neededCollAmt:   sdk.NewInt(103),                 //  602.5 - 500 -> 103 required
			expectedPass:    true,
		},
		{
			name:            "under-collateralized; truncated integer amount; non-unit price",
			protocolColl:    sdk.NewInt(500),
			priceCollStable: sdk.MustNewDecFromStr("0.999"), // startCollUSD = 500 * 0.999 -> 499.5
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.603"), // 0.603 * 1000 = 603
			//  603 - 499.5 = 103.5 -> 104 required
			neededCollAmt: sdk.NewInt(104),
			expectedPass:  true,
		},
	}

	for _, testCase := range expectedPasses {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, tc.targetCollRatio))
			require.NoError(t, nibiruApp.BankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(
					sdk.NewCoin(denoms.USDC, tc.protocolColl),
					sdk.NewCoin(denoms.NUSD, tc.stableSupply),
				),
			))

			// Post the price
			pair := asset.Registry.Pair(denoms.USDC, denoms.NUSD)
			nibiruApp.OracleKeeper.SetPrice(ctx, pair, tc.priceCollStable)

			neededCollAmount, err := stablecoinKeeper.RecollateralizeCollAmtForTargetCollRatio(ctx)
			if tc.expectedPass {
				require.NoError(t, err)
				require.EqualValues(t, tc.neededCollAmt, neededCollAmount)
			} else {
				require.Error(t, err)
			}
		})
	}

	expectedFails := []TestCaseRecollateralizeCollAmtForTargetCollRatio{
		{
			name:            "error from price not being posted",
			protocolColl:    sdk.NewInt(500),
			priceCollStable: sdk.OneDec(), // startCollUSD = 500 * 1 -> 500
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.6"), // 0.6 * 1000 = 600
			neededCollAmt:   sdk.NewInt(100),              // = 600 - 500
			expectedPass:    false,
		},
	}

	for _, testCase := range expectedFails {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, tc.targetCollRatio))
			require.NoError(t, nibiruApp.BankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(
					sdk.NewCoin(denoms.USDC, tc.protocolColl),
					sdk.NewCoin(denoms.NUSD, tc.stableSupply),
				),
			))

			// Post the price
			// pair := asset.AssetRegistry.Pair(denoms.USDC, denoms.NUSD)
			// nibiruApp.OracleKeeper.SetPrice(ctx, pair, tc.priceCollStable)

			neededCollAmount, err := stablecoinKeeper.RecollateralizeCollAmtForTargetCollRatio(ctx)
			if tc.expectedPass {
				require.NoError(t, err)
				require.EqualValues(t, tc.neededCollAmt, neededCollAmount)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGovAmtFromFullRecollateralize(t *testing.T) {
	testCases := []struct {
		name             string
		protocolColl     sdk.Int
		priceCollStable  sdk.Dec
		priceGovStable   sdk.Dec
		stableSupply     sdk.Int
		targetCollRatio  sdk.Dec
		postedAssetPairs []asset.Pair

		govOut       sdk.Int
		expectedPass bool
	}{
		{
			name:             "no prices posted",
			protocolColl:     sdk.NewInt(500),
			stableSupply:     sdk.NewInt(1000),
			targetCollRatio:  sdk.MustNewDecFromStr("0.6"),
			postedAssetPairs: []asset.Pair{},
			govOut:           sdk.Int{},
			expectedPass:     false,
		},
		{
			name:            "only post collateral price",
			protocolColl:    sdk.NewInt(500),
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.6"), // 0.6 * 1000 = 600
			priceCollStable: sdk.OneDec(),
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD)},
			govOut:       sdk.Int{},
			expectedPass: false,
		},
		{
			name:            "only post gov price",
			protocolColl:    sdk.NewInt(500),
			stableSupply:    sdk.NewInt(1000),
			targetCollRatio: sdk.MustNewDecFromStr("0.6"), // 0.6 * 1000 = 600
			priceGovStable:  sdk.OneDec(),
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD)},
			govOut:       sdk.Int{},
			expectedPass: false,
		},
		{
			name:            "correct computation - positive",
			protocolColl:    sdk.NewInt(5_000),
			stableSupply:    sdk.NewInt(10_000),
			targetCollRatio: sdk.MustNewDecFromStr("0.7"), // 0.7 * 10_000 = 7_000
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			priceCollStable: sdk.OneDec(),
			priceGovStable:  sdk.NewDec(2),
			// govOut = neededUSD * (1 + bonusRate) / priceGov
			//        = 2000 * (1.002) / 2 = 1002
			govOut:       sdk.NewInt(1002),
			expectedPass: true,
		},
		{
			name:            "correct computation - positive, new price",
			protocolColl:    sdk.NewInt(50_000),
			stableSupply:    sdk.NewInt(100_000),
			targetCollRatio: sdk.MustNewDecFromStr("0.7"), // 0.7 * 100_000 = 70_000
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			priceCollStable: sdk.OneDec(),
			priceGovStable:  sdk.NewDec(10),
			// govOut = neededUSD * (1 + bonusRate) / priceGov
			//        = 20000 * (1.002) / 10 = 2004
			govOut:       sdk.NewInt(2004),
			expectedPass: true,
		},
		{
			name:            "correct computation - negative",
			protocolColl:    sdk.NewInt(70_000),
			stableSupply:    sdk.NewInt(100_000),
			targetCollRatio: sdk.MustNewDecFromStr("0.5"), // 0.5 * 100_000 = 50_000
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			priceCollStable: sdk.OneDec(),
			priceGovStable:  sdk.NewDec(10),
			// govOut = neededUSD * (1 + bonusRate) / priceGov
			//        = -20000 * (1.002) / 10 = 2004
			govOut:       sdk.NewInt(-2004),
			expectedPass: true,
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, tc.targetCollRatio))
			require.NoError(t, nibiruApp.BankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(
					sdk.NewCoin(denoms.USDC, tc.protocolColl),
					sdk.NewCoin(denoms.NUSD, tc.stableSupply),
				),
			))

			prices := map[asset.Pair]sdk.Dec{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD): tc.priceGovStable,
				asset.Registry.Pair(denoms.USDC, denoms.NUSD): tc.priceCollStable,
			}
			for _, pair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, pair, prices[pair])
			}

			// Post prices to each specified market with the oracle.
			prices = map[asset.Pair]sdk.Dec{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD): tc.priceCollStable,
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD): tc.priceGovStable,
			}
			for _, assetPair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, assetPair, prices[assetPair])
			}

			govOut, err := stablecoinKeeper.GovAmtFromFullRecollateralize(ctx)
			if tc.expectedPass {
				require.NoError(t, err)
				require.EqualValues(t, tc.govOut, govOut)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Buyback and Recollateralize Tests
// ---------------------------------------------------------------------------

type NeededCollScenario struct {
	protocolColl    sdk.Int
	priceCollStable sdk.Dec
	stableSupply    sdk.Int
	collRatio       sdk.Dec
}

func (scenario NeededCollScenario) CalcNeededUSD() (neededUSD sdk.Dec) {
	stableUSD := scenario.collRatio.MulInt(scenario.stableSupply)
	collUSD := scenario.priceCollStable.MulInt(scenario.protocolColl)
	return stableUSD.Sub(collUSD)
}

func TestRecollateralize(t *testing.T) {
	testCases := []struct {
		name         string
		expectedPass bool
		err          error

		postedAssetPairs  []asset.Pair
		scenario          NeededCollScenario
		priceGovStable    sdk.Dec
		expectedNeededUSD sdk.Dec
		accFunds          sdk.Coins

		msg      types.MsgRecollateralize
		response *types.MsgRecollateralizeResponse
	}{
		{
			name: "both prices are $1",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(500_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD =  (0.6 * 1000e3) - (500e3 *1) = 100_000
			},
			priceGovStable: sdk.OneDec(),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.USDC, 1_000*common.TO_MICRO),
			),

			expectedNeededUSD: sdk.NewDec(100_000),
			msg: types.MsgRecollateralize{
				Creator: testutil.AccAddress().String(),
				Coll:    sdk.NewCoin(denoms.USDC, sdk.NewInt(100_000)),
			},
			response: &types.MsgRecollateralizeResponse{
				/*
					Gov.Amount = inCollUSD * (1 + bonusRate) / priceGovStable
					  = 100_000 * (1.002) / priceGovStable
					  = 100_200 / priceGovStable
				*/
				Gov: sdk.NewCoin(denoms.NIBI, sdk.NewInt(100_200)),
			},
			expectedPass: true,
		},
		{
			name: "arbitrary valid prices",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(500_000),
				priceCollStable: sdk.MustNewDecFromStr("1.099999"),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.7"),
				// neededUSD =  (0.7 * 1000e3) - (500e3 *1.09999) = 150_000.5
			},
			priceGovStable: sdk.NewDec(5),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.USDC, 1_000*common.TO_MICRO),
			),

			expectedNeededUSD: sdk.MustNewDecFromStr("150000.5"),
			msg: types.MsgRecollateralize{
				Creator: testutil.AccAddress().String(),
				Coll:    sdk.NewCoin(denoms.USDC, sdk.NewInt(50_000)),
			},
			response: &types.MsgRecollateralizeResponse{
				/*
					Gov.Amount = inCollUSD * (1 + bonusRate) / priceGovStable
					  = msg.Coll.Amount * priceCollStable (1.002) / priceGovStable
					  = 50_000 * 1.099999 * (1.002) / priceGovStable
					  = 55109.9499 / priceGovStable
					  = 11021.98998 -> 11_021
				*/
				Gov: sdk.NewCoin(denoms.NIBI, sdk.NewInt(11_021)),
			},
			expectedPass: true,
		},
		{
			name: "protocol has sufficient collateral - error",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			priceGovStable: sdk.NewDec(1),
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(500),
				priceCollStable: sdk.MustNewDecFromStr("1.099999"),
				stableSupply:    sdk.NewInt(1_000),
				collRatio:       sdk.MustNewDecFromStr("0.5"),
				// neededUSD =  (0.5 * 1000) - (500 *1.09999) = -49.9995
			},
			expectedNeededUSD: sdk.MustNewDecFromStr("-49.9995"),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.USDC, 1*common.TO_MICRO),
			),

			// Since 'neededUSD' is
			msg: types.MsgRecollateralize{
				Creator: testutil.AccAddress().String(),
				Coll:    sdk.NewCoin(denoms.USDC, sdk.NewInt(100)),
			},
			expectedPass: false,
			err:          fmt.Errorf("protocol has sufficient COLL"),
		},
		{
			name: "caller is broke - error",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			priceGovStable: sdk.NewDec(1),
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(500),
				priceCollStable: sdk.MustNewDecFromStr("1.5"),
				stableSupply:    sdk.NewInt(1_000),
				collRatio:       sdk.MustNewDecFromStr("0.9"),
				// neededUSD =  (0.9 * 1000) - (500 * 1.5) = 150
			},
			expectedNeededUSD: sdk.MustNewDecFromStr("150"),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.USDC, 99),
			),

			// Since 'neededUSD' is
			msg: types.MsgRecollateralize{
				Creator: testutil.AccAddress().String(),
				Coll:    sdk.NewCoin(denoms.USDC, sdk.NewInt(200)),
			},
			expectedPass: false,
			err:          fmt.Errorf("Not enough balance"),
		},
		{
			name: "negative msg.Coll.Amount - error",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			priceGovStable: sdk.NewDec(1),
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(500),
				priceCollStable: sdk.MustNewDecFromStr("1"),
				stableSupply:    sdk.NewInt(1_000),
				collRatio:       sdk.MustNewDecFromStr("0.9"),
				// neededUSD =  (0.9 * 1000) - (500 * 1) = 400
			},
			expectedNeededUSD: sdk.MustNewDecFromStr("400"),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.USDC, 400),
			),

			msg: types.MsgRecollateralize{
				Creator: testutil.AccAddress().String(),
				Coll:    sdk.Coin{Denom: denoms.USDC, Amount: sdk.NewInt(-200)},
			},
			expectedPass: false,
			err: fmt.Errorf(
				"collateral input, -200%v, must be positive", denoms.USDC),
		},
		{
			name: "oracle prices are expired - error",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			priceGovStable: sdk.NewDec(1),
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(500),
				priceCollStable: sdk.MustNewDecFromStr("1"),
				stableSupply:    sdk.NewInt(1_000),
				collRatio:       sdk.MustNewDecFromStr("0.9"),
				// neededUSD =  (0.9 * 1000) - (500 * 1) = 400
			},
			expectedNeededUSD: sdk.MustNewDecFromStr("400"),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.USDC, 400),
			),
			msg: types.MsgRecollateralize{
				Creator: testutil.AccAddress().String(),
				Coll:    sdk.NewInt64Coin(denoms.USDC, 400),
			},

			expectedPass: false,
			err:          fmt.Errorf("collections: not found"),
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			require.EqualValues(t, tc.expectedNeededUSD, tc.scenario.CalcNeededUSD())

			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, tc.scenario.collRatio))
			require.NoError(t, nibiruApp.BankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(
					sdk.NewCoin(denoms.USDC, tc.scenario.protocolColl),
					sdk.NewCoin(denoms.NUSD, tc.scenario.stableSupply),
				),
			))
			// Fund account
			caller, err := sdk.AccAddressFromBech32(tc.msg.Creator)
			if tc.expectedPass {
				require.NoError(t, err)
			}
			err = testapp.FundAccount(nibiruApp.BankKeeper, ctx, caller, tc.accFunds)
			if tc.expectedPass {
				require.NoError(t, err)
			}

			prices := map[asset.Pair]sdk.Dec{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD): tc.priceGovStable,
				asset.Registry.Pair(denoms.USDC, denoms.NUSD): tc.scenario.priceCollStable,
			}
			for _, pair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, pair, prices[pair])
			}

			// Post prices to each specified market with the oracle.
			prices = map[asset.Pair]sdk.Dec{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD): tc.scenario.priceCollStable,
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD): tc.priceGovStable,
			}
			for _, assetPair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, assetPair, prices[assetPair])
			}

			goCtx := sdk.WrapSDKContext(ctx)
			response, err := stablecoinKeeper.Recollateralize(goCtx, &tc.msg)
			if tc.expectedPass {
				require.NoError(t, err)
				require.EqualValues(t, tc.response, response)
			} else {
				assert.Error(t, err)
				require.ErrorContains(t, err, tc.err.Error())
			}
		},
		)
	}
}

func TestRecollateralize_Short(t *testing.T) {
	testCases := []struct {
		name string
		test func()
	}{
		{
			name: "invalid address - error",
			test: func() {
				nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
				goCtx := sdk.WrapSDKContext(ctx)

				msg := &types.MsgRecollateralize{
					Creator: "invalid-address",
				}
				_, err := nibiruApp.StablecoinKeeper.Recollateralize(goCtx, msg)
				require.Error(t, err)
			},
		},
		{
			name: "prices expired - error",
			test: func() {
				nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
				goCtx := sdk.WrapSDKContext(ctx)
				sender := testutil.AccAddress()
				msg := &types.MsgRecollateralize{
					Creator: sender.String(),
					Coll:    sdk.NewInt64Coin(denoms.USDC, 100),
				}
				_, err := nibiruApp.StablecoinKeeper.Recollateralize(goCtx, msg)
				require.ErrorContains(t, err, "collections: not found")
			},
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			tc.test()
		})
	}
}

func TestBuyback_MsgFormat(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		caller string
		gov    sdk.Coin
		err    error
	}{
		{
			name:   "regular invalid address",
			caller: "invalid_address",
			err:    fmt.Errorf("decoding bech32 failed: invalid separator index "),
		},
		{
			name:   "non-bech32 caller has invalid address for the msg",
			caller: "nibi_non_bech32",
			err:    fmt.Errorf("decoding bech32 failed: invalid separator index "),
		}, {
			name:   "valid creator address",
			caller: testutil.AccAddress().String(),
			err:    nil,
		},
	} {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			msg := types.MsgBuyback{
				Creator: tc.caller,
				Gov:     tc.gov,
			}

			_, err := nibiruApp.StablecoinKeeper.Buyback(
				sdk.WrapSDKContext(ctx),
				&msg,
			)

			require.Error(t, err)
			if tc.err != nil {
				require.Contains(t, err.Error(), tc.err.Error())
			}
		})
	}
}

func TestBuyback(t *testing.T) {
	testCases := []struct {
		name         string
		expectedPass bool

		postedAssetPairs      []asset.Pair
		scenario              NeededCollScenario
		priceGovStable        sdk.Dec
		expectedNeededUSD     sdk.Dec
		accFunds              sdk.Coins
		expectedAccFundsAfter sdk.Coins

		msg      types.MsgBuyback
		response *types.MsgBuybackResponse
	}{
		{
			name: "both prices are $1",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 1*common.TO_MICRO),
			),
			expectedAccFundsAfter: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 900_000), // accFunds - inGov.Amount
				sdk.NewInt64Coin(denoms.USDC, 100_000), // response.Coll
			),

			expectedNeededUSD: sdk.NewDec(-100_000),
			msg: types.MsgBuyback{
				Creator: testutil.AccAddress().String(),
				Gov:     sdk.NewCoin(denoms.NIBI, sdk.NewInt(100_000)),
			},
			response: &types.MsgBuybackResponse{
				/*
					Coll.Amount = inUSD *  / priceCollStable
					  = 100_000 / priceCollStable
				*/
				Coll: sdk.NewCoin(denoms.USDC, sdk.NewInt(100_000)),
			},
			expectedPass: true,
		},
		{
			name: "arbitrary valid prices",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(850_000),
				priceCollStable: sdk.MustNewDecFromStr("1.099999"),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.7"),
				// neededUSD =  (0.7 * 1000e3) - (850e3 *1.09999) = -234999.15
			},
			priceGovStable: sdk.NewDec(5),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 1*common.TO_MICRO),
			),
			expectedAccFundsAfter: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 953_000), // accFunds - inGov.Amount
				sdk.NewInt64Coin(denoms.USDC, 213_636), // response.Coll
			),

			expectedNeededUSD: sdk.MustNewDecFromStr("-234999.15"),
			msg: types.MsgBuyback{
				Creator: testutil.AccAddress().String(),
				Gov:     sdk.NewCoin(denoms.NIBI, sdk.NewInt(50_000)),
			},
			response: &types.MsgBuybackResponse{
				/*
					neededGovAmt = neededUSD.neg() / priceGovStable
					inGov.Amount = min(msg.Gov.Amount, neededGovAmt)
					  = min(47_000, 50_000)
					Coll.Amount = inUSD  / priceCollStable
					  = (inGov.Amount * priceGovStable)  / priceCollStable
					  = 47000 * 5 / 1.099999
					  = 213636.55785141626 -> 213_636
				*/
				Coll: sdk.NewCoin(denoms.USDC, sdk.NewInt(213_636)),
			},
			expectedPass: true,
		},
		{
			name: "msg has more NIBI than the protocol needs, only needed sent",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 1*common.TO_MICRO),
			),
			expectedAccFundsAfter: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 900_000), // accFunds - inGov.Amount
				sdk.NewInt64Coin(denoms.USDC, 100_000), // response.Coll
			),

			expectedNeededUSD: sdk.NewDec(-100_000),
			msg: types.MsgBuyback{
				Creator: testutil.AccAddress().String(),
				Gov:     sdk.NewCoin(denoms.NIBI, sdk.NewInt(200_000)),
			},
			response: &types.MsgBuybackResponse{
				// Coll.Amount = inUSD *  / priceCollStable
				Coll: sdk.NewCoin(denoms.USDC, sdk.NewInt(100_000)),
			},
			expectedPass: true,
		},
		{
			name: "protocol under-collateralized, so buyback won't run",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.8"),
				// neededUSD = (0.8 * 1000e3) - (700e3 *1) = 100_000
			},
			priceGovStable: sdk.OneDec(),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 1_000*common.TO_MICRO),
			),

			expectedNeededUSD: sdk.NewDec(100_000),
			msg: types.MsgBuyback{
				Creator: testutil.AccAddress().String(),
				Gov:     sdk.NewCoin(denoms.NIBI, sdk.NewInt(100_000)),
			},
			response:     &types.MsgBuybackResponse{},
			expectedPass: false,
		},
		{
			name: "caller has insufficient funds",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 1),
			),

			expectedNeededUSD: sdk.NewDec(-100_000),
			msg: types.MsgBuyback{
				Creator: testutil.AccAddress().String(),
				Gov:     sdk.NewCoin(denoms.NIBI, sdk.NewInt(100_000)),
			},
			response:     &types.MsgBuybackResponse{},
			expectedPass: false,
		},
		{
			name: "fail: missing collateral price post",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 1_000*common.TO_MICRO),
			),

			expectedNeededUSD: sdk.NewDec(-100_000),
			msg: types.MsgBuyback{
				Creator: testutil.AccAddress().String(),
				Gov:     sdk.NewCoin(denoms.NIBI, sdk.NewInt(100_000)),
			},
			response:     &types.MsgBuybackResponse{},
			expectedPass: false,
		},
		{
			name: "fail: missing NIBI price post",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(denoms.NIBI, 1_000*common.TO_MICRO),
			),

			expectedNeededUSD: sdk.NewDec(-100_000),
			msg: types.MsgBuyback{
				Creator: testutil.AccAddress().String(),
				Gov:     sdk.NewCoin(denoms.NIBI, sdk.NewInt(100_000)),
			},
			response:     &types.MsgBuybackResponse{},
			expectedPass: false,
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			require.EqualValues(t, tc.expectedNeededUSD, tc.scenario.CalcNeededUSD())

			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, tc.scenario.collRatio))

			// Fund module account based on scenario
			require.NoError(t, nibiruApp.BankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(
					sdk.NewCoin(denoms.USDC, tc.scenario.protocolColl),
					sdk.NewCoin(denoms.NUSD, tc.scenario.stableSupply),
				),
			))

			// Fund caller account
			caller, err := sdk.AccAddressFromBech32(tc.msg.Creator)
			if tc.expectedPass {
				require.NoError(t, err)
			}
			err = testapp.FundAccount(nibiruApp.BankKeeper, ctx, caller, tc.accFunds)
			if tc.expectedPass {
				require.NoError(t, err)
			}

			// Set up markets for the oracle keeper.
			prices := map[asset.Pair]sdk.Dec{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD): tc.priceGovStable,
				asset.Registry.Pair(denoms.USDC, denoms.NUSD): tc.scenario.priceCollStable,
			}
			for _, pair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, pair, prices[pair])
			}

			// Post prices to each specified market with the oracle.
			for _, assetPair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, assetPair, prices[assetPair])
			}

			goCtx := sdk.WrapSDKContext(ctx)
			response, err := stablecoinKeeper.Buyback(goCtx, &tc.msg)
			if tc.expectedPass {
				require.NoError(t, err)
				require.EqualValues(t, tc.response, response)
				require.EqualValues(t,
					tc.expectedAccFundsAfter,
					nibiruApp.BankKeeper.GetAllBalances(ctx, caller))
			} else {
				require.Error(t, err)
			}
		},
		)
	}
}

func TestBuybackGovAmtForTargetCollRatio(t *testing.T) {
	testCases := []struct {
		name         string
		scenario     NeededCollScenario
		expectedPass bool

		postedAssetPairs []asset.Pair
		priceGovStable   sdk.Dec

		outGovAmt sdk.Int
	}{
		{
			name: "both prices $1, correct amount out",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			outGovAmt:      sdk.NewInt(100_000),
			expectedPass:   true,
		},
		{
			name:             "both prices $1, correct amount out, no prices",
			postedAssetPairs: []asset.Pair{},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			outGovAmt:      sdk.NewInt(100_000),
			expectedPass:   false,
		},
		{
			name: "both prices $1, only coll price posted",
			postedAssetPairs: []asset.Pair{
				asset.Registry.Pair(denoms.USDC, denoms.NUSD),
			},
			scenario: NeededCollScenario{
				protocolColl:    sdk.NewInt(700_000),
				priceCollStable: sdk.OneDec(),
				stableSupply:    sdk.NewInt(1 * common.TO_MICRO),
				collRatio:       sdk.MustNewDecFromStr("0.6"),
				// neededUSD = (0.6 * 1000e3) - (700e3 *1) = -100_000
			},
			priceGovStable: sdk.OneDec(),
			outGovAmt:      sdk.NewInt(99_000),
			expectedPass:   false,
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			stablecoinKeeper := &nibiruApp.StablecoinKeeper
			require.NoError(t, stablecoinKeeper.SetCollRatio(ctx, tc.scenario.collRatio))
			require.NoError(t, nibiruApp.BankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(
					sdk.NewCoin(denoms.USDC, tc.scenario.protocolColl),
					sdk.NewCoin(denoms.NUSD, tc.scenario.stableSupply),
				),
			))

			prices := map[asset.Pair]sdk.Dec{
				asset.Registry.Pair(denoms.NIBI, denoms.NUSD): tc.priceGovStable,
				asset.Registry.Pair(denoms.USDC, denoms.NUSD): tc.scenario.priceCollStable,
			}
			for _, pair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, pair, prices[pair])
			}

			// Post prices to each specified market with the oracle.
			for _, assetPair := range tc.postedAssetPairs {
				nibiruApp.OracleKeeper.SetPrice(ctx, assetPair, prices[assetPair])
			}

			outGovAmt, err := stablecoinKeeper.BuybackGovAmtForTargetCollRatio(ctx)
			if tc.expectedPass {
				require.NoError(t, err)
				require.EqualValues(t, tc.outGovAmt, outGovAmt)
			} else {
				require.Error(t, err)
			}
		},
		)
	}
}
