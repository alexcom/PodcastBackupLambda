package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"os"
)

func NewArchive(table string) (*Archive, error) {
	region := os.Getenv("AWS_REGION")
	config := aws.NewConfig()
	config.Region = aws.String(region)
	s, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}
	client := dynamodb.New(s)
	return &Archive{client, table}, nil
}

type Archive struct {
	dynamodb *dynamodb.DynamoDB
	table    string
}

func (a *Archive) Append(filename, podcast string) {
	fmt.Printf("adding \"%s\" to podcast \"%s\" archive\n", filename, podcast)
	input := &dynamodb.PutItemInput{}
	input.SetTableName(a.table)
	input.SetItem(map[string]*dynamodb.AttributeValue{
		"filename": {
			S: aws.String(filename),
		},
		"podcast": {
			S: aws.String(podcast),
		},
	})

	_, err := a.dynamodb.PutItem(input)
	if err != nil {
		fmt.Println(err)
	}
}

func (a *Archive) Exists(filename, podcast string) (bool, error) {
	input := &dynamodb.GetItemInput{}
	input.SetTableName(a.table)
	input.SetKey(map[string]*dynamodb.AttributeValue{
		"filename": {
			S: aws.String(filename),
		},
	})
	item, err := a.dynamodb.GetItem(input)
	if err != nil {
		fmt.Println(err)
	}
	return err == nil && item != nil && item.Item != nil, err
}
