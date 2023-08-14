package main

import (
	"PodcastBackupLambda/meg"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type PodcastEvent struct {
	FeedURL         string `json:"feed_url"`
	TargetDirectory string `json:"target_directory"`
	Podcast         string `json:"podcast"`
}

func Handle(ctx context.Context, event PodcastEvent) (response string, err error) {
	archive, err := NewArchive(os.Getenv("ARCHIVE_TABLE"))
	if err != nil {
		return "error creating archive", err
	}
	email := os.Getenv("MEGA_EMAIL")
	password := os.Getenv("MEGA_PASSWORD")

	megaClient, err := meg.NewMegaClient(email, password)
	if err != nil {
		return "error creating mega client", err
	}
	destDir := event.TargetDirectory
	fmt.Println("destination location : ", destDir)
	err = megaClient.ChDir(destDir)
	if err != nil {
		return "", fmt.Errorf("path Lookup failed: %w", err)
	}

	links, err := processFeed(event.FeedURL)
	if err != nil {
		return "failure", err
	}

	successes, failures := processLinks(event.Podcast, links, megaClient, archive)

	if len(successes) == 0 {
		fmt.Println("no files were backed up")
	}

	return report(failures, successes), nil
}

func report(failures []string, successes []string) string {
	message := makeMessage(failures, successes)
	fmt.Println(message)

	if len(message) > 0 {
		sendEmailNotification(message)
	}
	return message
}

func processLinks(podcast string, links []string, megaClient *meg.MegaClient, archive *Archive) (successes []string, failures []string) {
	for _, podcastLink := range links {
		filename := podcastLink[strings.LastIndex(podcastLink, "/")+1:]
		fmt.Println("filename : ", filename)

		if exists, err := archive.Exists(filename, podcast); err == nil {
			if exists {
				fmt.Printf("file %s recorded in archive, skipping\n", filename)
				continue
			}
		} else {
			fmt.Printf("error checking file %s for existense, skipping: %s\n", filename, err)
			failures = append(failures, err.Error())
			continue
		}

		tempFilePath, err := downloadMediaToTempFile(podcastLink)
		fmt.Println("source temp file path : ", tempFilePath)

		err = megaClient.Upload(tempFilePath, filename)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			successes = append(successes, filename)
			archive.Append(filename, podcast)
		}
		_ = os.Remove(tempFilePath)
	}
	return successes, failures
}

func makeMessage(failures []string, successes []string) string {
	sb := strings.Builder{}
	if len(failures) > 0 {
		sb.WriteString("Failures:\n")
		for _, f := range failures {
			sb.WriteString(f)
			sb.WriteString("\n")
		}
		sb.WriteString("\n\n")
	}
	if len(successes) > 0 {
		sb.WriteString("Successfully backed up : \n")
		for _, s := range successes {
			sb.WriteString(s)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func downloadMediaToTempFile(podcastLink string) (filename string, err error) {
	fmt.Println("trying to download media : ", podcastLink)
	resp, err := http.Get(podcastLink)
	if err != nil {
		fmt.Println("failed to download media file", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code while downloading media file is %d, exiting", resp.StatusCode)
	}
	tempFile, err := ioutil.TempFile("", "podcast")
	if err != nil {
		return "", err
	}
	written, err := io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", err
	}
	err = tempFile.Sync()
	if err != nil {
		return "", err
	}
	fmt.Println("bytes copied to temp file: ", written)
	return tempFile.Name(), nil
}

func processFeed(feedURL string) ([]string, error) {
	fmt.Println("Refreshing feed", feedURL)
	client := http.Client{}
	resp, err := client.Get(feedURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code while feching feed is %d, exiting", resp.StatusCode)
	}
	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	rss := RSS{}
	err = xml.Unmarshal(bts, &rss)
	if err != nil {
		return nil, err
	}
	if rss.Channel != nil && len(rss.Channel) > 0 {
		var links []string
		for _, item := range rss.Channel[0].Items {
			if item.Enclosure != nil && item.Enclosure.Url != "" {
				links = append(links, item.Enclosure.Url)
			} else if item.MediaContent != nil && item.MediaContent.Url != "" {
				links = append(links, item.MediaContent.Url)
			}
		}
		fmt.Printf("found %d links\n", len(links))
		return links, nil
	}
	return nil, errors.New("feed channel entry is empty")
}

type RSS struct {
	Channel []struct {
		Items []struct {
			Title     string `xml:"title"`
			Enclosure *struct {
				Url string `xml:"url,attr"`
			} `xml:"enclosure"`
			MediaContent *struct {
				Url string `xml:"url,attr"`
			}
		} `xml:"item"`
	} `xml:"channel"`
}

func sendEmailNotification(textBody string) {
	email := os.Getenv("NOTIFICATION_EMAIL")
	//subject := "PodcastBackup notification"
	//fmt.Println("Email Sent to address: " + email)
	fmt.Println("NOT SENDING eMailto address : " + email + ". Reason: unimplemented")
}

func main() {
	if os.Getenv("LOCAL_DEBUG") == "true" {
		_, err := Handle(nil, PodcastEvent{FeedURL: "https://feeds.feedburner.com/HollywoodBabbleOnPod", TargetDirectory: "HollywoodBabbleOnBackup", Podcast: "Hollywood Babble-On"})
		if err != nil {
			println(err.Error())
		}
	} else {
		lambda.Start(Handle)
	}
}
