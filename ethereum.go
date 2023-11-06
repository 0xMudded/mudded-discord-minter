package main

import (
	"context"
	"errors"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	CHAIN_ID = 1
)

type TransactionCache struct {
	pending *types.Transaction
}

var transactionCache TransactionCache

func GetBalance() (string, error) {
	log.Println("Fetching balance...")

	ctx := context.Background()
	url := os.Getenv("RPC_URL")
	address := common.HexToAddress(os.Getenv("ADDRESS"))

	client, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return "", err
	}
	defer client.Close()

	balance, err := client.BalanceAt(ctx, address, nil)
	if err != nil {
		return "", err
	}

	return (WeiToEther(balance)).String(), nil
}

func ParseTransactionFromHash(hexHash string) (*types.Transaction, error) {
	log.Println("Parsing quicktask...")

	ctx := context.Background()
	url := os.Getenv("RPC_URL")
	hash := common.HexToHash(hexHash)

	client, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	tx, _, err := client.TransactionByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	transactionCache.pending = tx

	go func() {
		time.Sleep(10 * time.Minute)
		transactionCache.pending = nil
	}()

	return tx, nil
}

func ClearTransaction() {
	log.Println("Clearing transaction cache...")
	transactionCache.pending = nil
}

func SendPendingTransaction() (string, error) {
	log.Println("Sending pending transaction...")

	if transactionCache.pending == nil {
		return "", errors.New("no pending transactions")
	}

	ctx := context.Background()
	url := os.Getenv("RPC_URL")

	client, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return "", err
	}
	defer client.Close()

	address := common.HexToAddress(os.Getenv("ADDRESS"))
	nonce, err := client.PendingNonceAt(ctx, address)
	if err != nil {
		return "", err
	}

	pendingTx := transactionCache.pending
	tx := types.NewTransaction(nonce, *pendingTx.To(), pendingTx.Value(), pendingTx.Gas(), pendingTx.GasPrice(), pendingTx.Data())
	privateKey, err := crypto.HexToECDSA(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		return "", err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(CHAIN_ID)), privateKey)
	if err != nil {
		return "", err
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", err
	}

	return signedTx.Hash().Hex(), nil
}
