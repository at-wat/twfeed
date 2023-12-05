package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	tableName = "twfeed_data"
)

type TwFeedData struct {
	User        string `dynamodbav:"user"`
	LastFetched string `dynamodbav:"last_fetched"`
	Since       string `dynamodbav:"since"`
}

type db struct {
	cli *dynamodb.Client
}

func newDB(ctx context.Context) (*db, error) {
	c, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &db{
		cli: dynamodb.NewFromConfig(c),
	}, nil
}

func (d *db) GetLastFetched(ctx context.Context, user string) (string, string, error) {
	res, err := d.cli.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user": &types.AttributeValueMemberS{
				Value: user,
			},
		},
	})
	if err != nil {
		return "", "", err
	}
	var data TwFeedData
	if err := attributevalue.UnmarshalMap(res.Item, &data); err != nil {
		return "", "", err
	}
	return data.LastFetched, data.Since, nil
}

func (d *db) PutLastFetched(ctx context.Context, user, lastFetched, since string) error {
	item, err := attributevalue.MarshalMap(&TwFeedData{
		User:        user,
		LastFetched: lastFetched,
		Since:       since,
	})
	if err != nil {
		return err
	}
	_, err = d.cli.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	return err
}
