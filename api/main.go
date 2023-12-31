package main

import (
	"os"
	"fmt"
	"log"
	"sync"
	"time"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type APIResponse struct {
	Message  string `json:"message"`
}

type Response events.APIGatewayProxyResponse

var sqsClient *sqs.Client

const layout  string = "2006-01-02 15:04"
const layout2 string = "20060102150405.000"

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var jsonBytes []byte
	var err error
	d := make(map[string]string)
	json.Unmarshal([]byte(request.Body), &d)
	if v, ok := d["action"]; ok {
		switch v {
		case "sendmessage" :
			if m, ok := d["message"]; ok {
				e := sendMessage(ctx, m)
				if e != nil {
					err = e
				} else {
					jsonBytes, _ = json.Marshal(APIResponse{Message: "Success. Please Receive."})
				}
			}
		case "getcount" :
			message, e := getCount(ctx)
			if e != nil {
				err = e
			} else {
				jsonBytes, _ = json.Marshal(APIResponse{Message: message})
			}
		case "receivemessage" :
			message, e := receiveMessage(ctx)
			if e != nil {
				err = e
			} else {
				jsonBytes, _ = json.Marshal(APIResponse{Message: message})
			}
		}
	}
	log.Print(request.RequestContext.Identity.SourceIP)
	if err != nil {
		log.Print(err)
		jsonBytes, _ = json.Marshal(APIResponse{Message: fmt.Sprint(err)})
		return Response{
			StatusCode: 500,
			Body: string(jsonBytes),
		}, nil
	}
	return Response {
		StatusCode: 200,
		Body: string(jsonBytes),
	}, nil
}

func sendMessage(ctx context.Context, message string) error {
	t := time.Now()
	if sqsClient == nil {
		sqsClient = getSqsClient(ctx)
	}
	params := &sqs.SendMessageInput{
		MessageBody:            aws.String(message),
		QueueUrl:               aws.String(os.Getenv("QUEUE_URL")),
		MessageGroupId:         aws.String(os.Getenv("MESSAGE_GROUP_ID")),
		MessageDeduplicationId: aws.String(t.Format(layout2)),
	}
	_, err := sqsClient.SendMessage(ctx, params)
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func getCount(ctx context.Context)(string, error) {
	if sqsClient == nil {
		sqsClient = getSqsClient(ctx)
	}
	params := &sqs.GetQueueAttributesInput{
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages},
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
	}
	res, err := sqsClient.GetQueueAttributes(ctx, params)
	if err != nil {
		return "", err
	}
	return res.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessages)], nil
}

func deleteMessage(ctx context.Context, msg types.Message) error {
	if sqsClient == nil {
		sqsClient = getSqsClient(ctx)
	}
	params := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(os.Getenv("QUEUE_URL")),
		ReceiptHandle: msg.ReceiptHandle,
	}
	_, err := sqsClient.DeleteMessage(ctx, params)

	if err != nil {
		return err
	}
	return nil
}

func receiveMessage(ctx context.Context)(string, error) {
	if sqsClient == nil {
		sqsClient = getSqsClient(ctx)
	}
	params := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds: 3,
	}
	res, err := sqsClient.ReceiveMessage(ctx, params)
	if err != nil {
		return "", err
	}
	if len(res.Messages) == 0 {
		return "Empty.", nil
	}
	var wg sync.WaitGroup
	for _, m := range res.Messages {
		wg.Add(1)
		go func(msg types.Message) {
			defer wg.Done()
			if err := deleteMessage(ctx, msg); err != nil {
				log.Println(err)
			}
		}(m)
	}
	wg.Wait()
	return aws.ToString(res.Messages[0].Body), nil
}

func getSqsClient(ctx context.Context) *sqs.Client {
	return sqs.NewFromConfig(getConfig(ctx))
}

func getConfig(ctx context.Context) aws.Config {
	var err error
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		log.Print(err)
	}
	return cfg
}

func main() {
	lambda.Start(HandleRequest)
}
