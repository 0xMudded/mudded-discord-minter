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

type RequestWrapper struct {
	session *discordgo.Session
	message *discordgo.MessageCreate
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("no .env file found...")
	}
}

func (r *RequestWrapper) sendMessage(msg string) {
	_, err := r.session.ChannelMessageSend(r.message.ChannelID, msg)
	if err != nil {
		log.Printf("Error: %v", err)
	}
}

func (r *RequestWrapper) sendErrorMessage(err error) {
	msg := fmt.Sprintf("Error: %v", err)
	r.sendMessage(msg)
}

func (r *RequestWrapper) handleGetBalance() {
	balance, err := GetBalance()
	if err != nil {
		log.Printf("Error: %v", err)
		r.sendErrorMessage(err)
		return
	}

	formattedBalance := fmt.Sprintf("Account balance: %se", balance)
	r.sendMessage(formattedBalance)
}

func (r *RequestWrapper) handleQuickTask() {
	splitMessage := strings.Fields(r.message.Content)[1:]
	if len(splitMessage) < 1 {
		errMsg := errors.New("incorrect number of arguments")
		r.sendErrorMessage(errMsg)
		return
	}

	txHash := splitMessage[0]
	tx, err := ParseTransactionFromHash(txHash)
	if err != nil {
		log.Printf("Error: %v", err)
		r.sendErrorMessage(err)
		return
	}

	cost := WeiToEther(tx.Cost())
	confirmMessage := fmt.Sprintf("Transaction will cost approximately %fe. Would you like to proceed? (y/n)", cost)
	r.sendMessage(confirmMessage)
}

func (r *RequestWrapper) handleClear() {
	ClearTransaction()
}

func (r *RequestWrapper) handleTransaction() {
	hash, err := SendPendingTransaction()
	if err != nil {
		log.Printf("Error: %v", err)
		r.sendErrorMessage(err)
	}

	msg := fmt.Sprintf("Transaction sent! Check the status here: https://etherscan.io/tx/%s", hash)
	r.sendMessage(msg)
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID != os.Getenv("DISCORD_ID") || (!strings.HasPrefix(m.Content, "!") && (strings.ToLower(m.Content) != "n" && strings.ToLower(m.Content) != "y")) {
		return
	}

	requestWrapper := &RequestWrapper{
		session: s,
		message: m,
	}

	msgContent := strings.ToLower(m.Content)

	if strings.HasPrefix(msgContent, "!balance") {
		requestWrapper.handleGetBalance()
	}

	if strings.HasPrefix(msgContent, "!qt") {
		requestWrapper.handleQuickTask()
	}

	if msgContent == "n" {
		requestWrapper.handleClear()
	}

	if msgContent == "y" {
		requestWrapper.handleTransaction()
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
