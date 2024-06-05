package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func main() {
	uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)

	if len(os.Args) < 2 {
		fmt.Println("Expected 'upload' subcommand")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "upload":
		if err := uploadCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println("Failed to parse upload command")
			os.Exit(1)
		}
		if uploadCmd.NArg() != 1 {
			fmt.Println("You must provide a video file")
			os.Exit(1)
		}
		videoPath := uploadCmd.Arg(0)
		uploadVideo(videoPath)
	default:
		fmt.Println("Expected 'upload' subcommand")
		os.Exit(1)
	}
}

func uploadVideo(videoPath string) {
	ctx := context.Background()

	// Load your OAuth2.0 client credentials from file
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// Create an OAuth2.0 config
	config, err := google.ConfigFromJSON(b, youtube.YoutubeUploadScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	// Obtain an OAuth2.0 token
	client := getClient(ctx, config)

	// Create a YouTube service
	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	// Open the video file
	file, err := os.Open(videoPath)
	if err != nil {
		log.Fatalf("Error opening video file: %v", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Unable to get file info: %v", err)
	}

	fileName := fileInfo.Name()
	videoTitle := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Create a video resource
	video := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       videoTitle,
			Description: "",
			Tags:        []string{},
			CategoryId:  "22", // See https://developers.google.com/youtube/v3/docs/videoCategories/list
		},
		Status: &youtube.VideoStatus{PrivacyStatus: "private"},
	}

	fmt.Printf("Uploading %s\n", videoTitle)

	// Create progress bar
	bar := pb.Full.Start64(fileInfo.Size())
	reader := bar.NewProxyReader(file)

	// Upload the video
	call := service.Videos.Insert([]string{"snippet", "status"}, video)
	response, err := call.Media(reader).Do()
	if err != nil {
		log.Fatalf("Error making YouTube API call: %v", err)
	}

	bar.Finish()

	fmt.Printf("Upload successful! Video ID: %v\n", response.Id)

	err = os.Remove(videoPath)
	if err != nil {
		log.Fatalf("Error removing video: %v", err)
	}
}

func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	// Use a token file to store and retrieve OAuth 2.0 tokens
	tokenFile := "token.json"

	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	} else {
		// Check if the token is expired
		if tok.Expiry.Before(time.Now()) {
			tokenSource := config.TokenSource(ctx, tok)
			tok, err = tokenSource.Token()
			if err != nil {
				log.Fatalf("Unable to refresh token: %v", err)
			}
			saveToken(tokenFile, tok)
		}
	}
	return config.Client(ctx, tok)
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
