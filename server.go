package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)


// List of Application Variables
var (
	emailRegexp = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

// User return list of email and password found on the database
type User struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Pnum     int    `json:"pnum"`
}

//----[ FUNCTIONS ]------------------

// Validate email agains regex
func validateEmail(email string) bool {
	if !emailRegexp.MatchString(email) {
		return false
	}
	return true
}

// hidePassword gets the passworld lenght and first and last two chart and return the hidden password
func hidePassword(password string, pnum int) string {
	hiddenPassword := password[0:2]

	for i := 0; i < pnum-4; i++ {
		hiddenPassword += "*"
	}

	hiddenPassword += password[2:4]

	return hiddenPassword
}

// uniqueSlice gets a slice of passwords and return only unique values
func uniqueSlice(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, "\""+entry+"\"")
		}
	}
	return list
}

// sliceToJSON gets the password slices and return as a json string
func sliceToJSON(pass []string) string {
	passwords := uniqueSlice(pass)
	return strings.Join(passwords, ",")
}

// searchEmail searches for the email in the database
func searchEmail(email string, db *sql.DB) []string {

	// Get email first letter
	letter := returnFirstChar(email)

	// Execute the query
	query := fmt.Sprintf("SELECT email, password, pnum FROM %s WHERE email=\"%s\"", letter, email)
	results, err := db.Query(query)
	if err != nil {
		panic(err.Error())
	}

	var output []string

	// Iterate between all passwords and add it to the slice
	for results.Next() {
		var user User
		// for each row, scan the result into our tag composite object
		err = results.Scan(&user.Email, &user.Password, &user.Pnum)
		if err != nil {
			panic(err.Error())
		}
		// log.Printf("%s %s", user.Email, user.Password)
		output = append(output, hidePassword(user.Password, user.Pnum))
	}
	// Return slice
	return output
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

//----[ FUNCTIONS ]------------------

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
	readToken := os.Getenv("READTOKEN")
	//----- Environment Variables ---------//

	// Create new echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.LoggerWithConfig(
		middleware.LoggerConfig{Format: "=> method=${method}, status=${status}, time=${time_rfc3339_nano}, IP=${remote_ip}, agent=${user_agent}, latency=${latency_human}.\n\n"})
	)
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://pwdquery.xyz", "https://pwdquery.xyz"},
		AllowMethods: []string{echo.GET, echo.POST},
	}))

	// ROUTES
	e.GET("/", func(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
		return c.String(http.StatusOK, "{\"message\":\"Zup?!\"}")
	})

	// Check if email is in database
	e.GET("/query/:readToken/:email", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, "application/json")
		token := c.Param("readToken")
		email := c.Param("email")

		// Check if token match
		if token != readToken {
			fmt.Printf("=> Request token doesn't match. Variables Provided: token=>%s, email=>%s", token, email)
			return c.String(http.StatusBadRequest, "{ \"error\": \"Wrong token provided.\"}")
		}

		// Check if email is valid
		if !validateEmail(email) {
			fmt.Printf("=> Email is not valid. Variables provided token=>%s, email=>%s", token, email)
			return c.String(http.StatusBadRequest, "{ \"error\": \"Invalid email provided.\"}")
		}

		// Initiate database connection
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbServer, dbPort, dbDatabase))
		if err != nil {
			fmt.Println(err.Error())
			return c.String(http.StatusInternalServerError, "{ \"error\": \"Database error.\"}")
		}
		defer db.Close() // Avoid closing db connection

		// Get all passwords with that email address
		passwords := searchEmail(email, db)
		uniquePasswords := uniqueSlice(passwords)

		// Close db connection
		db.Close()

		// If no passwords found, great
		if len(passwords) == 0 {
			fmt.Printf("=> #0 [%s] ", email)
			return c.String(http.StatusOK, "{\"error\":\"Email not found.\"}")
		}

		// Display a list of passwords
		fmt.Printf("=> #%d [%s] %s ", len(uniquePasswords), email, uniquePasswords)
		return c.String(http.StatusOK, "{\"email\": \""+email+"\", \"password\":["+sliceToJSON(passwords)+"]}")
	})

	// Start echo server
	e.Logger.Fatal(e.Start(":4141"))

}
