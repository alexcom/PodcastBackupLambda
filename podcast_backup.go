package main

import (
	"PodcastBackupLambda/meg"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/t3rm1n4l/go-mega"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type PodcastEvent struct {
	FeedURL         string `json:"feed_url"`
	TargetDirectory string `json:"target_directory"`
}

func Handle(ctx context.Context, event PodcastEvent) (string, error) {
	m, err := createMega()
	if err != nil {
		return "", err
	}
	links, err := fetchFeed(event.FeedURL)
	if err != nil {
		return "failure", err
	}

	var failures []string
	var successes []string
	destDir := event.TargetDirectory
	fmt.Println("destination location : ", destDir)
	pathNodes, err := meg.ResolvePathOnMega(m, destDir)
	if err != nil {
		return "", fmt.Errorf("path Lookup failed: %w", err)
	}
	destNode := pathNodes[len(pathNodes)-1]
	for _, podcastLink := range links {
		filename := podcastLink[strings.LastIndex(podcastLink, "/")+1:]
		fmt.Println("filename : ", filename)

		if exists, err := meg.CheckExists(m, destNode, filename); err == nil {
			if exists {
				fmt.Printf("file %s already exists, skipping\n", filename)
				continue
			}
		} else {
			fmt.Printf("error checking file %s for existense, skipping: %s\n", filename, err)
			continue
		}

		tempFilePath, err := downloadMediaToTempFile(podcastLink)
		fmt.Println("source temp file path : ", tempFilePath)

		err = meg.UploadToMega(m, destNode, tempFilePath, filename)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			successes = append(successes, filename)
		}
		_ = os.Remove(tempFilePath)
	}
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
	fmt.Println(sb.String())
	fmt.Println("no files were backed up")
	if sb.Len() > 0 {
		sendEmailNotification(sb.String())
	}
	return sb.String(), nil
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

func createMega() (*mega.Mega, error) {
	email := os.Getenv("MEGA_EMAIL")
	password := os.Getenv("MEGA_PASSWORD")
	if email == "" || password == "" {
		return nil, errors.New("username or password empty. set env MEGA_EMAIL & MEGA_PASSWORD")
	}
	m := mega.New()
	err := m.Login(email, password)
	if err != nil {
		return nil, fmt.Errorf("login failed : %w", err)
	}
	return m, nil
}

func fetchFeed(feedURL string) ([]string, error) {
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
	awsSesZone := os.Getenv("AWS_SES_ZONE")
	subject := "PodcastBackup notification"
	s, err := session.NewSession(&aws.Config{Region: aws.String(awsSesZone)})
	if err != nil {
		fmt.Println(err)
		return
	}
	svc := ses.New(s)
	const encoding = "UTF-8"
	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: []*string{
				aws.String(email),
			}},
		Message: &ses.Message{
			Body: &ses.Body{
				Text: &ses.Content{
					Charset: aws.String(encoding),
					Data:    aws.String(textBody),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String(encoding),
				Data:    aws.String(subject),
			},
		},
		Source: aws.String(email),
	}
	output, err := svc.SendEmail(input)
	// error handling copied from AWS SES examples
	// Display error messages if they occur.
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ses.ErrCodeMessageRejected:
				fmt.Println(ses.ErrCodeMessageRejected, aerr.Error())
			case ses.ErrCodeMailFromDomainNotVerifiedException:
				fmt.Println(ses.ErrCodeMailFromDomainNotVerifiedException, aerr.Error())
			case ses.ErrCodeConfigurationSetDoesNotExistException:
				fmt.Println(ses.ErrCodeConfigurationSetDoesNotExistException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			fmt.Println(err.Error())
		}
		return
	}
	fmt.Println("Email Sent to address: " + email)
	fmt.Println(output)
}

func main() {
	if os.Getenv("LOCAL_DEBUG") == "true" {
		_, err := Handle(nil, PodcastEvent{FeedURL: "https://feeds.feedburner.com/HollywoodBabbleOnPod", TargetDirectory: "HollywoodBabbleOnBackup"})
		if err != nil {
			println(err.Error())
		}
	} else {
		lambda.Start(Handle)
	}
}
