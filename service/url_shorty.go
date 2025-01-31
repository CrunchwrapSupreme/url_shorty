package main

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	c "github.com/fatih/color"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type UrlEntry struct {
	ID int64        `json:"id"`
	ShortSlug string `json:"short_slug"`
	LongUrl string  `json:"long_url"`
	Protocol string `json:"protocol"`
	OwnerId int64
}

type TokenEntry struct {
	ID int64
	Token string
	OwnerId int64
}

func (entry *UrlEntry) MarshalJSON() ([]byte, error) {
	type Alias UrlEntry
	return json.Marshal(&struct {
		ShortUrl string `json:"short_url"`
		*Alias
	}{
		ShortUrl: fmt.Sprintf("stubhost.com/%s", entry.ShortSlug),
		Alias: (*Alias)(entry),
	})
}

func colorizeValue(text string) string {
	col := c.New(c.Underline)
	return col.Sprint(text)
}

func colorize(text string) string {
	var keywords = [...]string{"RESOLVED", "FOR"}
	var operators = [...]string{"->", "'", "slug", "(", ")", "dest"}
	for _, ele := range keywords {
		text = strings.ReplaceAll(text, ele, c.GreenString(ele))
	}
	for _, ele := range operators {
		text = strings.ReplaceAll(text, ele, c.CyanString(ele))
	}
	return text
}

func redirect_handler(w http.ResponseWriter, r *http.Request) {
	short_slug := r.PathValue("short_slug")
	long_url, err := shorty2tallE(short_slug)
	if err != nil {
		http.Error(w, err.Error(), 404)
		log.Println(err.Error())
	} else {
		h := w.Header()
		var lstr strings.Builder
		lstr.WriteString("RESOLVED")
		lstr.WriteString(
			fmt.Sprintf(
				" slug(%s) -> dest(%s)",
				short_slug,
				colorizeValue(long_url),
			),
		)
		lstr.WriteString(" FOR")
		lstr.WriteString(fmt.Sprintf(" host(%s, %s)", r.RemoteAddr, r.UserAgent()))
		log.Println(colorize(lstr.String()))
		h.Set("Location", long_url)
		w.WriteHeader(301)
	}
}


func new_url_handler(w http.ResponseWriter, r *http.Request) {
	var entry UrlEntry
	token := r.Header.Get("Authorization")
	token = strings.TrimSpace(token)
	row := db.QueryRow("SELECT * FROM auth_tokens WHERE token = ? LIMIT 1", token)
	err := row.Err()
	if err != nil {
		w.WriteHeader(401)
		return
	}
	has_json := strings.ToLower(r.Header.Get("Content-Type")) == "application/json"
	accepts := r.Header.Values("Accept")
	accepts_json := slices.IndexFunc(accepts, func(mtype string) bool {
		return strings.ToLower(mtype) == "application/json"
	}) > -1

	if !has_json {
		http.Error(w, "Must have Content-Type: application/json in request", 400)
		return
	} else if !accepts_json {
		http.Error(w, "Must have Accept: application/json in request", 400)
		return
	}
	err = json.NewDecoder(r.Body).Decode(&entry)
	if err != nil {
		http.Error(w, err.Error(), 400)
		log.Println(err.Error())
	}
	err = make_new_url(&entry)
	if err != nil {
		http.Error(w, err.Error(), 500)
		log.Println(err.Error())
	}

	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&entry)
	if err != nil {
		http.Error(w, err.Error(), 500)
		log.Println(err.Error())
	}
}

func make_new_url(entry *UrlEntry) error {
	if len(entry.LongUrl) > 255 {
		msg := fmt.Sprintf("long_url is irregular or too long '%s'", entry.LongUrl)
		return errors.New(msg)
	}
	_, err := url.Parse(entry.LongUrl)
	if err != nil {
		return err
	}
	short_slug, err := generate_short_slug()
	entry.ShortSlug = short_slug
	if err != nil {
		return err
	}
	var proto string
	switch entry.Protocol {
	case "":
		proto = "https"
	case "https":
		proto = "https"
	default:
		msg := fmt.Sprintf("Unknown 'protocol' %s", entry.Protocol)
		return errors.New(msg)
	}
	entry.Protocol = proto
	stmt := "INSERT INTO url_mappings (short_slug, long_url, protocol) VALUES (?, ?, ?)"
	result, err := db.Exec(stmt, entry.ShortSlug, entry.LongUrl, entry.Protocol)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	entry.ID = id
	return nil
}

func shorty2tallE(short_slug_slug string) (string, error) {
	var long_url string
	var protocol string
	row := db.QueryRow("SELECT long_url, protocol FROM url_mappings WHERE short_slug = ? LIMIT 1", short_slug_slug)
	err := row.Scan(&long_url, &protocol)
	if err != nil {
		return "", err
	}
	var built_url strings.Builder
	built_url.WriteString(protocol)
	built_url.WriteString("://")
	built_url.WriteString(long_url)
	return built_url.String(), nil
}

func generate_short_slug() (string, error) {
	data := make([]byte, 16)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	checksum := md5.Sum(data)
	encoded := base64.RawURLEncoding.EncodeToString(checksum[:8])
	return encoded, nil
}

func make_token_string() (string, error) {
	data := make([]byte, 32)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	checksum := sha256.Sum256(data)
	encoded := base64.RawURLEncoding.EncodeToString(checksum[:])
	return encoded, nil
}

func store_token(token_entry *TokenEntry, token string) error {
	token_entry.Token = token
	stmt := "INSERT INTO auth_tokens (token) VALUES (?)"
	result, err := db.Exec(stmt, &token_entry.Token)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	token_entry.ID = id
	return nil
}


func main() {
	var err error
	db, err = sql.Open("sqlite3", "./foo.db")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()
	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatalln(pingErr)
	}
	log.Println("Sqlite3 connected to file://foo.db")
	db.SetConnMaxIdleTime(5)
	db.SetConnMaxLifetime(10)
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(10)

	http.HandleFunc("GET /{short_slug}", redirect_handler)
	http.HandleFunc("POST /new-url", new_url_handler)
	log.Printf("HTTP server listening on %s", colorizeValue("http://127.0.0.1:7777"))
	log.Fatalln(http.ListenAndServe(":7777", nil))
}
