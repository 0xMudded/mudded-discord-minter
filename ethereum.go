package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	CHAIN_ID = 1
)

type SimulationPayload struct {
	Save           bool   `json:"save"`
	SaveIfFails    bool   `json:"save_if_fails"`
	SimulationType string `json:"simulation_type"`
	NetworkID      string `json:"network_id"`
	From           string `json:"from"`
	To             string `json:"to"`
	Input          string `json:"input"`
	Gas            uint64 `json:"gas"`
	GasPrice       int64  `json:"gas_price"`
	Value          int64  `json:"value"`
}

type SimulationResponse struct {
	Transaction TransactionResponse
}

type TransactionResponse struct {
	Status bool
}

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

	success, err := simulateTransaction()
	if err != nil {
		return nil, err
	}

	if !success {
		return nil, errors.New("execution reverted")
	}

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

func simulateTransaction() (bool, error) {
	log.Println("Simulating transaction...")

	url := getEndpoint()
	pendingTx := transactionCache.pending
	payload := &SimulationPayload{
		Save:           true,
		SaveIfFails:    false,
		SimulationType: "full",
		NetworkID:      strconv.Itoa(CHAIN_ID),
		From:           os.Getenv("ADDRESS"),
		To:             pendingTx.To().String(),
		Input:          "0x" + hex.EncodeToString(pendingTx.Data()),
		Gas:            pendingTx.Gas(),
		GasPrice:       pendingTx.GasPrice().Int64(),
		Value:          pendingTx.Value().Int64(),
	}

	body, err := doRequest(url, payload)
	if err != nil {
		return false, err
	}

	var simulationResponse SimulationResponse
	if err = json.Unmarshal(body, &simulationResponse); err != nil {
		return false, err
	}

	return simulationResponse.Transaction.Status, nil
}

func doRequest(url string, payload *SimulationPayload) ([]byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Access-Key", os.Getenv("TENDERLY_ACCESS_TOKEN"))
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func getEndpoint() string {
	return fmt.Sprintf("https://api.tenderly.co/api/v1/account/%s/project/%s/simulate", os.Getenv("TENDERLY_USER_NAME"), os.Getenv("TENDERLY_PROJECT_SLUG"))
}
