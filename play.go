package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"strings"

	"github.com/campoy/tools/imgcat"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

const redirectURI = "http://localhost:8080/callback"

var (
	clientID       = os.Getenv("CLIENT_ID")
	secretKey      = os.Getenv("SECRET_KEY")
	auth           = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadPrivate, spotify.ScopeUserReadPlaybackState, spotify.ScopeUserModifyPlaybackState)
	currentUser, _ = user.Current()
	state          = "playcli"
	tokenDir       = currentUser.HomeDir + "/spotify_token.json"
	ch             = make(chan bool)
	searchLimit    = 10
)

func main() {
	auth.SetAuthInfo(clientID, secretKey)
	loginFlag := flag.Bool("login", false, "Regenerate OAuth2 token")
	listFlag := flag.Bool("l", false, "List songs before playing")
	selectActivePlayerFlag := flag.Bool("p", false, "Select active player")

	flag.Parse()

	// If login flag set
	if *loginFlag {
		log.Println("Regenerating token")
		generateToken()
		os.Exit(0)
	}

	close(ch)

	tok, err := loadToken()

	var q string

	if err != nil {
		log.Println("No token detected. Please run with -login flag.")
		os.Exit(0)
	}

	client := auth.NewClient(tok)

	// If active player flag set
	if *selectActivePlayerFlag {
		selectActivePlayer(client)
		os.Exit(0)
	}

	if *listFlag {
		q = strings.Join(os.Args[2:], " ")
	} else {
		q = strings.Join(os.Args[1:], " ")
	}

	result, err := client.SearchOpt(q, spotify.SearchTypeTrack, &spotify.Options{
		Limit: &searchLimit,
	})
	if err != nil {
		log.Fatal(err)
	}

	for trackIndex, track := range result.Tracks.Tracks {
		if *listFlag {
			fmt.Println(trackIndex, ": "+track.Name+" by "+track.Artists[0].Name)
		}
	}
	fmt.Println("")

	selectedIndex := 0

	if *listFlag {
		fmt.Print("Select Song -> ")
		fmt.Scan(&selectedIndex)
	}

	song := result.Tracks.Tracks[selectedIndex]

	title := song.Name
	artist := song.Artists[0].Name
	album := song.Album.Name
	albumCover := song.Album.Images[1].URL
	uri := song.URI

	fmt.Println("♫ Playing", title, "by", artist, "("+album+")")

	PlayOptions := spotify.PlayOptions{
		URIs: []spotify.URI{
			uri,
		},
	}

	err = client.PlayOpt(&PlayOptions)

	if err != nil {
		fmt.Println(err)
	}

	// fmt.Println(albumCover)
	// // a := exec.Command("pixterm", "-s", "2", "-tr", "10", albumCover)
	// a := exec.Command("curl", "|", albumCover)
	// output, err := a.Output()

	// if err != nil {
	// 	fmt.Println(err)
	// }

	// fmt.Println(output)

	enc, err := imgcat.NewEncoder(os.Stdout, imgcat.Width(imgcat.Pixels(200)), imgcat.Inline(true), imgcat.Name(albumCover))
	if err != nil {
		log.Fatal(err)
	}

	f, err := http.Get(albumCover)
	if err != nil {
		log.Fatal(err)
	}
	// defer func() { _ = f.Close() }()

	// Display the image in the terminal.
	if err := enc.Encode(f.Body); err != nil {
		log.Fatal(err)
	}

	client.Play()

}

func generateToken() {
	http.HandleFunc("/callback", authCallback)
	go http.ListenAndServe(":8080", nil)
	url := auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	_ = <-ch

	fmt.Println("🔑 Saved token to " + tokenDir)
}

func authCallback(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}
	saveToken(tok)
	fmt.Fprintf(w, "<script>close();</script>")

	ch <- true
}

func selectActivePlayer(client spotify.Client) {
	devices, _ := client.PlayerDevices()

	for deviceID, device := range devices {
		fmt.Println(deviceID, ":", device.Name)
	}
	fmt.Println("")

	var selectedIndex int
	fmt.Print("Select Player -> ")
	fmt.Scan(&selectedIndex)

	client.TransferPlayback(devices[selectedIndex].ID, true)
}

// Adapted from https://developers.google.com/people/quickstart/go
func loadToken() (*oauth2.Token, error) {
	f, err := os.Open(tokenDir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(token *oauth2.Token) {
	f, err := os.OpenFile(tokenDir, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
