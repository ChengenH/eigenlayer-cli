package operator

import (
	"context"
	"fmt"
	"os"

	"github.com/Layr-Labs/eigenlayer-cli/pkg/common"
	"github.com/Layr-Labs/eigenlayer-cli/pkg/telemetry"
	"github.com/Layr-Labs/eigenlayer-cli/pkg/utils"

	elContracts "github.com/Layr-Labs/eigensdk-go/chainio/clients/elcontracts"
	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	"github.com/Layr-Labs/eigensdk-go/chainio/txmgr"
	eigensdkLogger "github.com/Layr-Labs/eigensdk-go/logging"
	eigenMetrics "github.com/Layr-Labs/eigensdk-go/metrics"

	gethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/urfave/cli/v2"
)

func UpdateCmd(p utils.Prompter) *cli.Command {
	updateCmd := &cli.Command{
		Name:      "update",
		Usage:     "Update the operator metadata onchain",
		UsageText: "update <configuration-file>",
		Description: `
		Updates the operator metadata onchain which includes 
			- metadata url
			- delegation approver address
			- earnings receiver address
			- staker opt out window blocks

		Requires the same file used for registration as argument
		`,
		After: telemetry.AfterRunAction(),
		Action: func(cCtx *cli.Context) error {
			args := cCtx.Args()
			if args.Len() != 1 {
				return fmt.Errorf("%w: accepts 1 arg, received %d", ErrInvalidNumberOfArgs, args.Len())
			}

			configurationFilePath := args.Get(0)
			operatorCfg, err := common.ValidateAndReturnConfig(configurationFilePath)
			if err != nil {
				return err
			}
			cCtx.App.Metadata["network"] = operatorCfg.ChainId.String()

			fmt.Printf(
				"\r%s Operator configuration file validated successfully %s\n",
				utils.EmojiCheckMark,
				operatorCfg.Operator.Address,
			)

			logger := eigensdkLogger.NewTextSLogger(os.Stdout, &eigensdkLogger.SLoggerOptions{})

			ethClient, err := eth.NewClient(operatorCfg.EthRPCUrl)
			if err != nil {
				return err
			}

			keyWallet, sender, err := common.GetWallet(
				operatorCfg.SignerConfig,
				operatorCfg.Operator.Address,
				ethClient,
				p,
				operatorCfg.ChainId,
				logger,
			)
			if err != nil {
				return err
			}

			txMgr := txmgr.NewSimpleTxManager(keyWallet, ethClient, logger, sender)

			noopMetrics := eigenMetrics.NewNoopMetrics()

			elWriter, err := elContracts.BuildELChainWriter(
				gethcommon.HexToAddress(operatorCfg.ELDelegationManagerAddress),
				gethcommon.HexToAddress(operatorCfg.ELAVSDirectoryAddress),
				ethClient,
				logger,
				noopMetrics,
				txMgr)

			if err != nil {
				return err
			}

			receipt, err := elWriter.UpdateOperatorDetails(context.Background(), operatorCfg.Operator)
			if err != nil {
				fmt.Printf("%s Error while updating operator details\n", utils.EmojiCrossMark)
				return err
			}
			fmt.Printf(
				"%s Operator details updated at: %s\n",
				utils.EmojiCheckMark,
				common.GetTransactionLink(receipt.TxHash.String(), &operatorCfg.ChainId),
			)
			common.PrintRegistrationInfo(
				"",
				gethcommon.HexToAddress(operatorCfg.Operator.Address),
				&operatorCfg.ChainId,
			)

			fmt.Printf(
				"%s Operator updated successfully. There is a 30 minute delay between update and operator details being shown in our webapp.\n",
				utils.EmojiCheckMark,
			)
			return nil
		},
	}

	return updateCmd
}
