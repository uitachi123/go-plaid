package main

import (
	"flag"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/uitachi123/go-plaid/pkg/api"
	"github.com/uitachi123/go-plaid/pkg/db"
	"github.com/uitachi123/go-plaid/pkg/echo"
	"github.com/uitachi123/go-plaid/pkg/plaid"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func setUpLogger(level string) *zap.Logger {
	l, err := zapcore.ParseLevel(strings.ToLower(level))
	if err != nil {
		panic(err)
	}
	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(l),
		Encoding:    "json",
		OutputPaths: []string{"stdout"},
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

func main() {

	loggingLevel := flag.String("logging", "INFO", "logging level")
	port := flag.String("port", "8080", "listening port")
	flag.Parse()

	logger := setUpLogger(*loggingLevel)
	defer logger.Sync()
	logger.Info("Starting web server...",
		// Structured context as strongly typed Field values.
		zap.String("time", time.Now().String()),
		zap.String("logging level", *loggingLevel),
	)

	_, err := db.Init()
	if err != nil {
		logger.Error("Error initializing database", zap.Error(err))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/echo/", echo.Echo)
	mux.HandleFunc("/users", api.Users)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "OK")
	})

	// endpoints for plaid
	mux.HandleFunc("/api/set_access_token", plaid.GetAccessToken)
	mux.HandleFunc("/api/create_link_token_for_payment", plaid.CreateLinkTokenForPayment)
	mux.HandleFunc("/api/auth", plaid.Auth)
	mux.HandleFunc("/api/accounts", plaid.Accounts)
	mux.HandleFunc("/api/balance", plaid.Balance)
	mux.HandleFunc("/api/item", plaid.Item)
	mux.HandleFunc("/api/identity", plaid.Identity)
	mux.HandleFunc("/api/transactions", plaid.Transactions)
	mux.HandleFunc("/api/payment", plaid.Payment)
	mux.HandleFunc("/api/create_public_token", plaid.CreatePublicToken)
	mux.HandleFunc("/api/create_link_token", plaid.CreateLinkToken)
	mux.HandleFunc("/api/investments_transactions", plaid.InvestmentTransactions)
	mux.HandleFunc("/api/holdings", plaid.Holdings)
	mux.HandleFunc("/api/assets", plaid.Assets)
	mux.HandleFunc("/api/transfer", plaid.Transfer)
	mux.HandleFunc("/api/info", plaid.Info)

	// listen to port
	http.ListenAndServe(":"+*port, mux)
}
