package plaid

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	plaid "github.com/plaid/plaid-go/v3/plaid"
)

var (
	PLAID_CLIENT_ID                      = ""
	PLAID_SECRET                         = ""
	PLAID_ENV                            = ""
	PLAID_PRODUCTS                       = ""
	PLAID_COUNTRY_CODES                  = ""
	PLAID_REDIRECT_URI                   = ""
	APP_PORT                             = ""
	client              *plaid.APIClient = nil
)

var environments = map[string]plaid.Environment{
	"sandbox":     plaid.Sandbox,
	"development": plaid.Development,
	"production":  plaid.Production,
}

func init() {
	// load env vars from .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error when loading environment variables from .env file %w", err)
	}

	// set constants from env
	PLAID_CLIENT_ID = os.Getenv("PLAID_CLIENT_ID")
	PLAID_SECRET = os.Getenv("PLAID_SECRET")

	if PLAID_CLIENT_ID == "" || PLAID_SECRET == "" {
		log.Fatal("Error: PLAID_SECRET or PLAID_CLIENT_ID is not set. Did you copy .env.example to .env and fill it out?")
	}

	PLAID_ENV = os.Getenv("PLAID_ENV")
	PLAID_PRODUCTS = os.Getenv("PLAID_PRODUCTS")
	PLAID_COUNTRY_CODES = os.Getenv("PLAID_COUNTRY_CODES")
	PLAID_REDIRECT_URI = os.Getenv("PLAID_REDIRECT_URI")
	APP_PORT = os.Getenv("APP_PORT")

	// set defaults
	if PLAID_PRODUCTS == "" {
		PLAID_PRODUCTS = "transactions"
	}
	if PLAID_COUNTRY_CODES == "" {
		PLAID_COUNTRY_CODES = "US"
	}
	if PLAID_ENV == "" {
		PLAID_ENV = "sandbox"
	}
	if APP_PORT == "" {
		APP_PORT = "8000"
	}
	if PLAID_CLIENT_ID == "" {
		log.Fatal("PLAID_CLIENT_ID is not set. Make sure to fill out the .env file")
	}
	if PLAID_SECRET == "" {
		log.Fatal("PLAID_SECRET is not set. Make sure to fill out the .env file")
	}

	// create Plaid client
	configuration := plaid.NewConfiguration()
	configuration.AddDefaultHeader("PLAID-CLIENT-ID", PLAID_CLIENT_ID)
	configuration.AddDefaultHeader("PLAID-SECRET", PLAID_SECRET)
	configuration.UseEnvironment(environments[PLAID_ENV])
	client = plaid.NewAPIClient(configuration)
}

// We store the access_token in memory - in production, store it in a secure
// persistent data store.
var accessToken string
var itemID string

var paymentID string

// The transfer_id is only relevant for the Transfer ACH product.
// We store the transfer_id in memory - in production, store it in a secure
// persistent data store
var transferID string

func GetAccessToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "Method not supported")
		return
	}
	publicToken := r.PostForm.Get("public_token")
	if publicToken == "" {
		io.WriteString(w, "Cant find public token")
		return
	}
	ctx := context.Background()

	// exchange the public_token for an access_token
	exchangePublicTokenResp, _, err := client.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(
		*plaid.NewItemPublicTokenExchangeRequest(publicToken),
	).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	accessToken = exchangePublicTokenResp.GetAccessToken()
	itemID = exchangePublicTokenResp.GetItemId()
	if itemExists(strings.Split(PLAID_PRODUCTS, ","), "transfer") {
		transferID, err = authorizeAndCreateTransfer(ctx, client, accessToken)
	}

	fmt.Println("public token: " + publicToken)
	fmt.Println("access token: " + accessToken)
	fmt.Println("item ID: " + itemID)

	b, err := json.Marshal(map[string]interface{}{
		"access_token": accessToken,
		"item_id":      itemID,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

// This functionality is only relevant for the UK Payment Initiation product.
// Creates a link token configured for payment initiation. The payment
// information will be associated with the link token, and will not have to be
// passed in again when we initialize Plaid Link.
func CreateLinkTokenForPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "Method not supported")
		return
	}
	ctx := context.Background()

	// Create payment recipient
	paymentRecipientRequest := plaid.NewPaymentInitiationRecipientCreateRequest("Harry Potter")
	paymentRecipientRequest.SetIban("GB33BUKB20201555555555")
	paymentRecipientRequest.SetAddress(*plaid.NewPaymentInitiationAddress(
		[]string{"4 Privet Drive"},
		"Little Whinging",
		"11111",
		"GB",
	))
	paymentRecipientCreateResp, _, err := client.PlaidApi.PaymentInitiationRecipientCreate(ctx).PaymentInitiationRecipientCreateRequest(*paymentRecipientRequest).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	// Create payment
	paymentCreateRequest := plaid.NewPaymentInitiationPaymentCreateRequest(
		paymentRecipientCreateResp.GetRecipientId(),
		"paymentRef",
		*plaid.NewPaymentAmount("GBP", 1.34),
	)
	paymentCreateResp, _, err := client.PlaidApi.PaymentInitiationPaymentCreate(ctx).PaymentInitiationPaymentCreateRequest(*paymentCreateRequest).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	paymentID = paymentCreateResp.GetPaymentId()
	fmt.Println("payment id: " + paymentID)

	linkTokenCreateReqPaymentInitiation := plaid.NewLinkTokenCreateRequestPaymentInitiation(paymentID)
	linkToken, err := linkTokenCreate(linkTokenCreateReqPaymentInitiation)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"link_token": linkToken,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Auth(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	authGetResp, _, err := client.PlaidApi.AuthGet(ctx).AuthGetRequest(
		*plaid.NewAuthGetRequest(accessToken),
	).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"accounts": authGetResp.GetAccounts(),
		"numbers":  authGetResp.GetNumbers(),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Accounts(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	accountsGetResp, _, err := client.PlaidApi.AccountsGet(ctx).AccountsGetRequest(
		*plaid.NewAccountsGetRequest(accessToken),
	).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"accounts": accountsGetResp.GetAccounts(),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Balance(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	balancesGetResp, _, err := client.PlaidApi.AccountsBalanceGet(ctx).AccountsBalanceGetRequest(
		*plaid.NewAccountsBalanceGetRequest(accessToken),
	).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"accounts": balancesGetResp.GetAccounts(),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Item(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	itemGetResp, _, err := client.PlaidApi.ItemGet(ctx).ItemGetRequest(
		*plaid.NewItemGetRequest(accessToken),
	).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	institutionGetByIdResp, _, err := client.PlaidApi.InstitutionsGetById(ctx).InstitutionsGetByIdRequest(
		*plaid.NewInstitutionsGetByIdRequest(
			*itemGetResp.GetItem().InstitutionId.Get(),
			convertCountryCodes(strings.Split(PLAID_COUNTRY_CODES, ",")),
		),
	).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"item":        itemGetResp.GetItem(),
		"institution": institutionGetByIdResp.GetInstitution(),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Identity(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	identityGetResp, _, err := client.PlaidApi.IdentityGet(ctx).IdentityGetRequest(
		*plaid.NewIdentityGetRequest(accessToken),
	).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"identity": identityGetResp.GetAccounts(),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Transactions(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Set cursor to empty to receive all historical updates
	var cursor *string

	// New transaction updates since "cursor"
	var added []plaid.Transaction
	var modified []plaid.Transaction
	var removed []plaid.RemovedTransaction // Removed transaction ids
	hasMore := true
	// Iterate through each page of new transaction updates for item
	for hasMore {
		request := plaid.NewTransactionsSyncRequest(accessToken)
		if cursor != nil {
			request.SetCursor(*cursor)
		}
		resp, _, err := client.PlaidApi.TransactionsSync(
			ctx,
		).TransactionsSyncRequest(*request).Execute()
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}

		// Add this page of results
		added = append(added, resp.GetAdded()...)
		modified = append(modified, resp.GetModified()...)
		removed = append(removed, resp.GetRemoved()...)
		hasMore = resp.GetHasMore()
		// Update cursor to the next cursor
		nextCursor := resp.GetNextCursor()
		cursor = &nextCursor
	}

	sort.Slice(added, func(i, j int) bool {
		return added[i].GetDate() < added[j].GetDate()
	})
	latestTransactions := added[len(added)-9:]

	b, err := json.Marshal(map[string]interface{}{
		"latest_transactions": latestTransactions,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

// This functionality is only relevant for the UK Payment Initiation product.
// Retrieve Payment for a specified Payment ID
func Payment(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	paymentGetResp, _, err := client.PlaidApi.PaymentInitiationPaymentGet(ctx).PaymentInitiationPaymentGetRequest(
		*plaid.NewPaymentInitiationPaymentGetRequest(paymentID),
	).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"payment": paymentGetResp,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

// This functionality is only relevant for the ACH Transfer product.
// Retrieve Transfer for a specified Transfer ID
func Transfer(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	transferGetResp, _, err := client.PlaidApi.TransferGet(ctx).TransferGetRequest(
		*plaid.NewTransferGetRequest(transferID),
	).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"transfer": transferGetResp.GetTransfer(),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func InvestmentTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	endDate := time.Now().Local().Format("2006-01-02")
	startDate := time.Now().Local().Add(-30 * 24 * time.Hour).Format("2006-01-02")

	request := plaid.NewInvestmentsTransactionsGetRequest(accessToken, startDate, endDate)
	invTxResp, _, err := client.PlaidApi.InvestmentsTransactionsGet(ctx).InvestmentsTransactionsGetRequest(*request).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"investments_transactions": invTxResp,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Holdings(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	holdingsGetResp, _, err := client.PlaidApi.InvestmentsHoldingsGet(ctx).InvestmentsHoldingsGetRequest(
		*plaid.NewInvestmentsHoldingsGetRequest(accessToken),
	).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"holdings": holdingsGetResp,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func Info(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(map[string]interface{}{
		"item_id":      itemID,
		"access_token": accessToken,
		"products":     strings.Split(PLAID_PRODUCTS, ","),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func CreatePublicToken(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Create a one-time use public_token for the Item.
	// This public_token can be used to initialize Link in update mode for a user
	publicTokenCreateResp, _, err := client.PlaidApi.ItemCreatePublicToken(ctx).ItemPublicTokenCreateRequest(
		*plaid.NewItemPublicTokenCreateRequest(accessToken),
	).Execute()

	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"public_token": publicTokenCreateResp.GetPublicToken(),
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func CreateLinkToken(w http.ResponseWriter, r *http.Request) {
	linkToken, err := linkTokenCreate(nil)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"link_token": linkToken,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func convertCountryCodes(countryCodeStrs []string) []plaid.CountryCode {
	countryCodes := []plaid.CountryCode{}

	for _, countryCodeStr := range countryCodeStrs {
		countryCodes = append(countryCodes, plaid.CountryCode(countryCodeStr))
	}

	return countryCodes
}

func convertProducts(productStrs []string) []plaid.Products {
	products := []plaid.Products{}

	for _, productStr := range productStrs {
		products = append(products, plaid.Products(productStr))
	}

	return products
}

// linkTokenCreate creates a link token using the specified parameters
func linkTokenCreate(
	paymentInitiation *plaid.LinkTokenCreateRequestPaymentInitiation,
) (string, error) {
	ctx := context.Background()
	countryCodes := convertCountryCodes(strings.Split(PLAID_COUNTRY_CODES, ","))
	products := convertProducts(strings.Split(PLAID_PRODUCTS, ","))
	redirectURI := PLAID_REDIRECT_URI

	user := plaid.LinkTokenCreateRequestUser{
		ClientUserId: time.Now().String(),
	}

	request := plaid.NewLinkTokenCreateRequest(
		"Plaid Quickstart",
		"en",
		countryCodes,
		user,
	)

	request.SetProducts(products)

	if redirectURI != "" {
		request.SetRedirectUri(redirectURI)
	}

	if paymentInitiation != nil {
		request.SetPaymentInitiation(*paymentInitiation)
	}

	linkTokenCreateResp, _, err := client.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*request).Execute()

	if err != nil {
		return "", err
	}

	return linkTokenCreateResp.GetLinkToken(), nil
}

func Assets(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// create the asset report
	assetReportCreateResp, _, err := client.PlaidApi.AssetReportCreate(ctx).AssetReportCreateRequest(
		*plaid.NewAssetReportCreateRequest([]string{accessToken}, 10),
	).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	assetReportToken := assetReportCreateResp.GetAssetReportToken()

	// get the asset report
	assetReportGetResp, err := pollForAssetReport(ctx, client, assetReportToken)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	// get it as a pdf
	pdfRequest := plaid.NewAssetReportPDFGetRequest(assetReportToken)
	pdfFile, _, err := client.PlaidApi.AssetReportPdfGet(ctx).AssetReportPDFGetRequest(*pdfRequest).Execute()
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	reader := bufio.NewReader(pdfFile)
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	// convert pdf to base64
	encodedPdf := base64.StdEncoding.EncodeToString(content)
	b, err := json.Marshal(map[string]interface{}{
		"json": assetReportGetResp.GetReport(),
		"pdf":  encodedPdf,
	})
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	io.WriteString(w, string(b))
}

func pollForAssetReport(ctx context.Context, client *plaid.APIClient, assetReportToken string) (*plaid.AssetReportGetResponse, error) {
	numRetries := 20
	request := plaid.NewAssetReportGetRequest(assetReportToken)

	for i := 0; i < numRetries; i++ {
		response, _, err := client.PlaidApi.AssetReportGet(ctx).AssetReportGetRequest(*request).Execute()
		if err != nil {
			plaidErr, err := plaid.ToPlaidError(err)
			if plaidErr.ErrorCode == "PRODUCT_NOT_READY" {
				time.Sleep(1 * time.Second)
				continue
			} else {
				return nil, err
			}
		} else {
			return &response, nil
		}
	}
	return nil, errors.New("Timed out when polling for an asset report.")
}

// This is a helper function to authorize and create a Transfer after successful
// exchange of a public_token for an access_token. The transfer_id is then used
// to obtain the data about that particular Transfer.
func authorizeAndCreateTransfer(ctx context.Context, client *plaid.APIClient, accessToken string) (string, error) {
	// We call /accounts/get to obtain first account_id - in production,
	// account_id's should be persisted in a data store and retrieved
	// from there.
	accountsGetResp, _, _ := client.PlaidApi.AccountsGet(ctx).AccountsGetRequest(
		*plaid.NewAccountsGetRequest(accessToken),
	).Execute()

	accountID := accountsGetResp.GetAccounts()[0].AccountId

	transferAuthorizationCreateUser := plaid.NewTransferUserInRequest("FirstName LastName")
	transferAuthorizationCreateRequest := plaid.NewTransferAuthorizationCreateRequest(
		accessToken,
		accountID,
		"credit",
		"ach",
		"1.34",
		"ppd",
		*transferAuthorizationCreateUser,
	)
	transferAuthorizationCreateResp, _, err := client.PlaidApi.TransferAuthorizationCreate(ctx).TransferAuthorizationCreateRequest(*transferAuthorizationCreateRequest).Execute()
	if err != nil {
		return "", err
	}
	authorizationID := transferAuthorizationCreateResp.GetAuthorization().Id

	transferCreateRequest := plaid.NewTransferCreateRequest(
		accessToken,
		accountID,
		authorizationID,
		"credit",
		"ach",
		"1.34",
		"Payment",
		"ppd",
		*transferAuthorizationCreateUser,
	)
	transferCreateResp, _, err := client.PlaidApi.TransferCreate(ctx).TransferCreateRequest(*transferCreateRequest).Execute()
	if err != nil {
		return "", err
	}

	return transferCreateResp.GetTransfer().Id, nil
}

// Helper function to determine if Transfer is in Plaid product array
func itemExists(array []string, product string) bool {
	for _, item := range array {
		if item == product {
			return true
		}
	}

	return false
}
