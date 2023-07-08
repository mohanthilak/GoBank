package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

type APIServer struct {
	store      Storage
	listenAddr string
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()
	router.HandleFunc("/account", makeHTTPHandleFunc(s.handleAccount))
	router.HandleFunc("/account/{id}", withJWTAuth(makeHTTPHandleFunc(s.handleGetAccountByID), s.store))
	router.HandleFunc("/transfer", makeHTTPHandleFunc(s.handleTransferAccount))
	log.Println("JSON API serving at port:", s.listenAddr)
	http.ListenAndServe(s.listenAddr, router)
}

func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetAccounts(w, r)
	}
	if r.Method == "POST" {
		return s.handleCreateAccount(w, r)
	}
	if r.Method == "DELETE" {
		return s.handleDeleteAccount(w, r)
	}
	return nil
}

func (s *APIServer) handleGetAccountByID(w http.ResponseWriter, r *http.Request) error {
	id, err := getIDFromReq(r)
	if err != nil {
		return err
	}

	if r.Method == "GET" {
		account, err := s.store.GetAccountByID(id)
		if err != nil {
			return err
		}
		return WriteJSON(w, http.StatusOK, account)
	}

	if r.Method == "DELETE" {
		err = s.store.DeleteAccount(id)
		log.Println(err)
		if err != nil {
			return err
		}
		return WriteJSON(w, http.StatusOK, map[string]int{"deleted": id})
	}

	return fmt.Errorf("method not valid %s", r.Method)
}

func (s *APIServer) handleGetAccounts(w http.ResponseWriter, r *http.Request) error {
	accounts, err := s.store.GetAccounts()
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, accounts)
}

func (s *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	createAccountReq := createAccountStruct{}
	if err := json.NewDecoder((r.Body)).Decode(&createAccountReq); err != nil {
		return err
	}
	account := NewAccount(createAccountReq.FirstName, createAccountReq.LastName)
	if err := s.store.CreateAccount(account); err != nil {
		return err
	}
	tokenString, err := createJWTToken(account)
	if err != nil {
		return err
	}
	log.Println(tokenString)
	return WriteJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := getIDFromReq(r)
	if err != nil {
		return err
	}
	err = s.store.DeleteAccount(id)
	log.Println(err)
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, map[string]int{"deleted": id})
}

func (s *APIServer) handleTransferAccount(w http.ResponseWriter, r *http.Request) error {
	transferReq := new(TransferRequest)
	err := json.NewDecoder(r.Body).Decode(transferReq)
	defer r.Body.Close()
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, transferReq)
}

func getIDFromReq(r *http.Request) (int, error) {
	id := mux.Vars(r)["id"]
	i, err := strconv.Atoi(id)
	if err != nil {
		return 0, fmt.Errorf("invalid ID given")
	}
	return i, nil
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func createJWTToken(account *Account) (string, error) {
	// timenow := time.Now().Local().Add(time.Minute * time.Duration(15)).Unix()
	claims := &jwt.MapClaims{
		"expiresAt":     15000,
		"accountNumber": account.Number,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func validateJWT(tokenString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")

	log.Println("Secret:", secret)
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("usnexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return []byte(secret), nil
	})
}

func permissionDenied(w http.ResponseWriter) {
	WriteJSON(w, http.StatusForbidden, ApiError{Error: "Permission Denied"})
}

func withJWTAuth(handlerFunc http.HandlerFunc, s Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Calling the JWT Auth func")

		tokenString := r.Header.Get("x-jwt-token")
		log.Println("tokenString: ", tokenString)
		if tokenString == "" {
			permissionDenied(w)
			return
		}
		token, err := validateJWT(tokenString)
		if err != nil {
			permissionDenied(w)
			return
		}
		if !token.Valid {
			permissionDenied(w)
			return
		}

		userId, err := getIDFromReq(r)
		if err != nil {
			permissionDenied(w)
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		log.Println(claims)

		account, err := s.GetAccountByID(userId)
		log.Println("account from DB: ", account, account.Number, claims["accountNumber"])
		if err != nil {
			permissionDenied(w)
			return
		}

		tokenAccountNumber := claims["accountNumber"]
		if account.Number != int64(tokenAccountNumber.(float64)) {
			log.Println("Account Number Not Matching", int64(tokenAccountNumber.(float64)), account.Number)

			permissionDenied(w)
			return
		}
		log.Println("Account Number Matched")
		handlerFunc(w, r)
	}
}

type apiFunc func(http.ResponseWriter, *http.Request) error
type ApiError struct {
	Error string `json:"error"`
}

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, ApiError{Error: err.Error()})
		}
	}
}
