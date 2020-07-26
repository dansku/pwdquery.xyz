package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	. "github.com/logrusorgru/aurora"
)

// List of Application Variables
var (
	emailRegexp = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

// User return list of email and password found on the database
type User struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

//----[ FUNCTIONS ]------------------

// validateEmailPassword checks if the email is valid
func validateEmailPassword(account string) bool {
	re := regexp.MustCompile("^(([^<>()\\[\\]\\.,;:\\s@\"]+(\\.[^<>()\\[\\]\\.,;:\\s@\"]+)*)|(\".+\"))@((\\[[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}])|(([a-zA-Z\\-0-9]+\\.)+[a-zA-Z]{2,}))+\\:(.*)$")
	return re.MatchString(account)
}

// hidePassword gets the entire password and returns the first and last two chars
func hidePassword(password string) string {
	splitPass := strings.Split(password, "")
	splitPassLen := len(splitPass)
	hiddenPassword := ""

	for key, value := range splitPass {
		if key == 0 || key == 1 || key == splitPassLen-1 || key == splitPassLen-2 {
			hiddenPassword += value
		}
	}

	return hiddenPassword
}

// returnFirstChar returns the first char of the string, if its not ascii or it is number, return with `other`
func returnFirstChar(email string) string {
	letters := [26]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	firstLetter := string([]rune(email)[0])

	for _, b := range letters {
		if b == firstLetter {
			return firstLetter
		}
	}
	return "other"
}

// insertIntoDatabase inserts the new email and password to the database
func insertIntoDatabase(filename string, letter string, email string, password string, pnum int, db *sql.DB) bool {
	// Execute database the query
	query := fmt.Sprintf("INSERT INTO %s(email, password, pnum) VALUES ('%s', '%s', '%d')", letter, email, password, pnum)
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("[%s/%s] %s.", email, password, err.Error())
		return false
	}
	return true
}

// checkAccount verifies if this email and password are already in the database
func checkAccount(filename string, i int, letter string, email string, password string, db *sql.DB) bool {
	// Execute database the query
	query := fmt.Sprintf("SELECT email, password FROM %s WHERE email='%s' AND password='%s'", letter, email, password)
	results, err := db.Query(query)
	if err != nil {
		log.Printf("%s - [%s/%s] Error %s.", filename, email, password, err.Error())
	}

	j := 0
	for results.Next() {
		j++
	}

	if j == 0 {
		// Account NOT found in database
		return true
	}
	return false
}

// clearString remove unnecessary chars from string
func clearString(message string) string {
	message = strings.Replace(message, "'", "", -1)
	message = strings.Replace(message, "\\", "", -1)
	return message
}

func main() {

	//----- Environment Variables ---------//
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dbUser := os.Getenv("DBUSER")
	dbPassword := os.Getenv("DBPASSWORD")
	dbServer := os.Getenv("DBSERVER")
	dbPort := os.Getenv("DBPORT")
	dbDatabase := os.Getenv("DBDATABASE")
	//----- Environment Variables ---------//

	// Initiate database connection
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbServer, dbPort, dbDatabase))

	if err != nil {
		fmt.Println(err.Error())
	}

	defer db.Close() // Avoid closing db connection

	// Confirms that there are 3 arguments [file, concurrency, start] otherwise stops the application
	if len(os.Args) != 4 {
		log.Fatal("You need to enter file.txt concurrency start")
	}

	// Reads arguments
	filename := os.Args[1]

	concurrency, err := strconv.ParseInt(os.Args[2], 10, 0)
	if err != nil {
		log.Fatal("Concurrency must be an integer.")
	}

	start, err := strconv.ParseInt(os.Args[3], 10, 0)
	if err != nil {
		log.Fatal("Start must be an integer.")
	}

	// Starts parsing the file
	fmt.Printf("Starting application, reading file %s with concurrency %d and starting at %d.\n", filename, concurrency, start)

	fptr := flag.String("fpath", filename, "file path to read from")
	flag.Parse()

	f, err := os.Open(*fptr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	s := bufio.NewScanner(f)
	i := 0
	newPassword := 0
	duplicatedPassword := 0
	tooShort := 0

	// Setup Concurrency
	maxGoroutines := concurrency
	guard := make(chan struct{}, maxGoroutines)

	// Scan all the lines
	for s.Scan() {
		i++

		// Starting point
		if i >= int(start) {

			// Check if regex passes
			if validateEmailPassword(s.Text()) {

				// Split string between email and password
				s := strings.Split(s.Text(), ":")
				email, password := strings.ToLower(clearString(s[0])), clearString(s[1])
				letter := returnFirstChar(email)
				passwordLenght := len(password)
				// Start workers to parse and save to database
				guard <- struct{}{}
				go func(n int) {
					insertIntoDatabase(filename, letter, email, password, passwordLenght, db)
					<-guard
				}(i)

			}
		}
	}

	err = s.Err()
	if err != nil {
		log.Fatal(err)
	}
}
