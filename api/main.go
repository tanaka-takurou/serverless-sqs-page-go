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
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type APIResponse struct {
	Message  string `json:"message"`
}

type Response events.APIGatewayProxyResponse

var cfg aws.Config
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
		sqsClient = sqs.New(cfg)
	}
	params := &sqs.SendMessageInput{
		MessageBody:            aws.String(message),
		QueueUrl:               aws.String(os.Getenv("QUEUE_URL")),
		MessageGroupId:         aws.String(os.Getenv("MESSAGE_GROUP_ID")),
		MessageDeduplicationId: aws.String(t.Format(layout2)),
	}
	req := sqsClient.SendMessageRequest(params)
	_, err := req.Send(ctx)
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func getCount(ctx context.Context)(string, error) {
	if sqsClient == nil {
		sqsClient = sqs.New(cfg)
	}
	params := &sqs.GetQueueAttributesInput{
		AttributeNames: []sqs.QueueAttributeName{sqs.QueueAttributeNameApproximateNumberOfMessages},
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
	}
	req := sqsClient.GetQueueAttributesRequest(params)
	res, err := req.Send(ctx)
	if err != nil {
		return "", err
	}
	return res.GetQueueAttributesOutput.Attributes[string(sqs.QueueAttributeNameApproximateNumberOfMessages)], nil
}

func deleteMessage(ctx context.Context, msg sqs.Message) error {
	if sqsClient == nil {
		sqsClient = sqs.New(cfg)
	}
	params := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(os.Getenv("QUEUE_URL")),
		ReceiptHandle: msg.ReceiptHandle,
	}
	req := sqsClient.DeleteMessageRequest(params)
	_, err := req.Send(ctx)

	if err != nil {
		return err
	}
	return nil
}

func receiveMessage(ctx context.Context)(string, error) {
	if sqsClient == nil {
		sqsClient = sqs.New(cfg)
	}
	params := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
		MaxNumberOfMessages: aws.Int64(1),
		WaitTimeSeconds: aws.Int64(3),
	}
	req := sqsClient.ReceiveMessageRequest(params)
	res, err := req.Send(ctx)
	if err != nil {
		return "", err
	}
	if len(res.ReceiveMessageOutput.Messages) == 0 {
		return "Empty.", nil
	}
	var wg sync.WaitGroup
	for _, m := range res.ReceiveMessageOutput.Messages {
		wg.Add(1)
		go func(msg sqs.Message) {
			defer wg.Done()
			if err := deleteMessage(ctx, msg); err != nil {
				log.Println(err)
			}
		}(m)
	}
	wg.Wait()
	return aws.StringValue(res.ReceiveMessageOutput.Messages[0].Body), nil
}

func init() {
	var err error
	cfg, err = external.LoadDefaultAWSConfig()
	cfg.Region = os.Getenv("REGION")
	if err != nil {
		log.Print(err)
	}
}

func main() {
	lambda.Start(HandleRequest)
}
