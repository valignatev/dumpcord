package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Author struct {
	Id       string
	Username string
}

type Attachment struct {
	Id           string
	Filename     string
	Size         float64
	Url          string
	Proxy_Url    string
	Width        float64
	Height       float64
	Content_Type string
}

type Post struct {
	Id          string
	Content     string
	Author      Author
	Attachments []Attachment
}

const baseUrl string = "https://discord.com/api/v9/channels/%s/messages"
const UA string = "Yoba Ethical Discord Backuper 101 Don't Ban Pls :3"

var (
	auth    string
	channel string
	before  string
	mode    string
)

func init() {
	flag.StringVar(&auth, "auth", "",
		"Auth token. Alternatively, create a file called auth without extension "+
			"and put your auth there",
	)
	flag.StringVar(&channel, "channel", "", "Channel to download (required)")
	flag.StringVar(&before, "before", "",
		"Oldest post in the channel to start from "+
			"(optional, default is start from newest post in the channel)",
	)
	flag.StringVar(&mode, "mode", "get-raw",
		"Mode of operation. Can be get-raw and parse. "+
			"get-raw downloads raw json and puts it in a folder, "+
			"parse reads those files and downloads attachments",
	)
}

func main() {
	flag.Parse()
	if channel == "" {
		flag.PrintDefaults()
		fmt.Printf("channel argument is required\n")
		os.Exit(1)
	}
	if auth == "" {
		// Try auth file
		authData, err := os.ReadFile("./auth")
		auth = strings.TrimSpace(string(authData))

		if err != nil {
			log.Fatalf("auth argument is empty and auth file could not be read: %s", err)
		}
	}

	if mode == "get-raw" {
		fmt.Println("Getting raw data...")
		getRaw()
	} else if mode == "parse" {
		fmt.Println("Parsing the raw data...")
		parse()
	} else {
		log.Fatalf("Incorrect mode: %s\n", mode)
	}
}

func getRaw() {
	channelUrl := fmt.Sprintf(baseUrl, channel)
	url, err := url.Parse(channelUrl)
	if err != nil {
		fmt.Printf("Url is wrong!\n")
		os.Exit(1)
	}

	query := url.Query()
	query.Set("limit", "100")
	if before != "" {
		query.Set("before", before)
	}
	url.RawQuery = query.Encode()

	rawScrapedDir := fmt.Sprintf("raw_%s", channel)
	err = os.MkdirAll(rawScrapedDir, 0755)
	if err != nil {
		log.Fatal("Failed to create a raw scrap directory")
	}

	for {
		fullUrl := url.String()
		fmt.Printf("Processing %s\n", fullUrl)
		req, err := http.NewRequest(http.MethodGet, fullUrl, nil)
		if err != nil {
			log.Fatalf("Couldn't create a request: %s\n", err)
		}
		req.Header.Set("Authorization", auth)
		req.Header.Set("User-Agent", UA)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("Error making http request to channel: %s\n", err)
		}

		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatalf("Couldn't read response body: %s\n", err)
		}

		var posts []Post
		err = json.Unmarshal(resBody, &posts)
		if err != nil {
			log.Fatalf("Error parsing json: %s\nData received is: %s\n", err, resBody)
		}

		if len(posts) > 0 {
			lastPost := posts[len(posts)-1].Id
			fileName := fmt.Sprintf("%s_%s.json", channel, lastPost)
			fullPath := filepath.Join(rawScrapedDir, fileName)

			err = os.WriteFile(fullPath, resBody, 0644)
			if err != nil {
				log.Fatalf("Couldn't write %s: %s\n", fullPath, err)
			}

			query.Set("before", lastPost)
			url.RawQuery = query.Encode()

		} else {
			fmt.Println("We're done!")
			break
		}
	}
}

func parse() {
	rawDirName := fmt.Sprintf("raw_%s", channel)
	parsedDir := fmt.Sprintf("parsed_%s", channel)
	err := os.MkdirAll(parsedDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create %s directory\n", parsedDir)
	}

	wg := sync.WaitGroup{}
	// Hardcode 10 simultaneous downloads at most
	var sem = make(chan int, 10)

	err = filepath.Walk(rawDirName, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if info.IsDir() {
			return nil
		}
		extension := filepath.Ext(path)
		if extension != ".json" {
			fmt.Printf("%s is not a .json file\n", path)
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var posts []Post
		err = json.Unmarshal(data, &posts)
		if err != nil {
			return err
		}

		for _, post := range posts {
			postDir := filepath.Join(parsedDir, post.Id)
			err = os.MkdirAll(postDir, 0755)
			if err != nil {
				log.Fatalf("Failed to create %s directory\n", postDir)
			}
			// TODO: post content can also just link discord CDN link
			// directly, need to parse it and download as well
			for _, attachment := range post.Attachments {
				go downloadAttachment(postDir, post.Id, attachment, &wg, sem)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error while parsing a file: %s\n", err)
	}

	wg.Wait()
	fmt.Println("Done parsing", rawDirName)
}

func downloadAttachment(baseDir string, postId string, attachment Attachment, wg *sync.WaitGroup, sem chan int) {
	wg.Add(1)
	defer wg.Done()

	sem <- 1

	fullPath := filepath.Join(baseDir, attachment.Filename)

	// Technically, this check is not enough to know if file already exists
	// but for our purpose it's good enough (tm)
	if _, err := os.Stat(fullPath); err == nil {
		// Apparently, you can't do `defer <- sem`, it fails with syntax error,
		// so I do it here
		<-sem
		return
	}

	fmt.Printf("Downloading %s\n", fullPath)
	req, err := http.NewRequest(http.MethodGet, attachment.Url, nil)
	if err != nil {
		log.Fatalf("Couldn't create a request: %s\n", err)
	}
	req.Header.Set("User-Agent", UA)

	client := http.Client{Timeout: 60 * time.Second}
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Fatalf("Couldn't download file: %s\nError is: %s\n", err)
	}
	out, err := os.Create(fullPath)
	defer out.Close()

	_, err = io.Copy(out, res.Body)
	if err != nil {
		fmt.Printf("Couldn't write to file %s: %s\n", fullPath, err)
		// Trying to cleanup so subsequent run can redownload it
		err := os.Remove(fullPath)
		if err != nil {
			log.Fatalf("Failed to cleanup the file: %s\n", fullPath)
		}
	}
	<-sem
}
