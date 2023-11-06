package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("no .env file found...")
	}
}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, msg string) {
	_, err := s.ChannelMessageSend(m.ChannelID, msg)
	if err != nil {
		log.Printf("Error: %v", err)
	}
}

func sendErrorMessage(s *discordgo.Session, m *discordgo.MessageCreate, err error) {
	msg := fmt.Sprintf("Error: %v", err)
	sendMessage(s, m, msg)
}

func handleGetBalance(s *discordgo.Session, m *discordgo.MessageCreate) {
	balance, err := GetBalance()
	if err != nil {
		log.Printf("Error: %v", err)
		sendErrorMessage(s, m, err)
	}

	formattedBalance := fmt.Sprintf("Account balance: %se", balance)
	sendMessage(s, m, formattedBalance)
}

func handleQuickTask(s *discordgo.Session, m *discordgo.MessageCreate) {
	splitMessage := strings.Fields(m.Content)[1:]
	if len(splitMessage) < 1 {
		errMsg := errors.New("incorrect number of arguments")
		sendErrorMessage(s, m, errMsg)
		return
	}

	txHash := splitMessage[0]
	tx, err := ParseTransactionFromHash(txHash)
	if err != nil {
		log.Printf("Error: %v", err)
		sendErrorMessage(s, m, err)
	}

	cost := WeiToEther(tx.Cost())
	confirmMessage := fmt.Sprintf("Transaction will cost approximately %fe. Would you like to proceed? (y/n)", cost)

	_, err = s.ChannelMessageSend(m.ChannelID, confirmMessage)
	if err != nil {
		log.Printf("Error: %v", err)
	}
}

func handleClear() {
	ClearTransaction()
}

func handleTransaction(s *discordgo.Session, m *discordgo.MessageCreate) {
	hash, err := SendPendingTransaction()
	if err != nil {
		log.Printf("Error: %v", err)
		sendErrorMessage(s, m, err)
	}

	msg := fmt.Sprintf("Transaction sent! Check the status here: https://etherscan.io/tx/%s", hash)
	sendMessage(s, m, msg)
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID != os.Getenv("DISCORD_ID") || (!strings.HasPrefix(m.Content, "!") && (strings.ToLower(m.Content) != "n" && strings.ToLower(m.Content) != "y")) {
		return
	}

	msgContent := strings.ToLower(m.Content)

	if strings.HasPrefix(msgContent, "!balance") {
		handleGetBalance(s, m)
	}

	if strings.HasPrefix(msgContent, "!qt") {
		handleQuickTask(s, m)
	}

	if msgContent == "n" {
		handleClear()
	}

	if msgContent == "y" {
		handleTransaction(s, m)
	}
}

func main() {
	s, err := discordgo.New("Bot " + os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	s.AddHandler(handleMessage)
	s.Identify.Intents = discordgo.IntentsGuildMessages

	err = s.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	log.Println("Bot is running...")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
