package main

import (
	"os"
	"fmt"
	"log"
	"sync"
	"time"
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type APIResponse struct {
	Message  string `json:"message"`
}

type Response events.APIGatewayProxyResponse

const layout         string = "2006-01-02 15:04"
const layout2        string = "20060102150405.000"

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var jsonBytes []byte
	var err error
	d := make(map[string]string)
	json.Unmarshal([]byte(request.Body), &d)
	if v, ok := d["action"]; ok {
		switch v {
		case "sendmessage" :
			if m, ok := d["message"]; ok {
				e := sendMessage(m)
				if e != nil {
					err = e
				} else {
					jsonBytes, _ = json.Marshal(APIResponse{Message: "Success. Please Receive."})
				}
			}
		case "getcount" :
			message, e := getCount()
			if e != nil {
				err = e
			} else {
				jsonBytes, _ = json.Marshal(APIResponse{Message: message})
			}
		case "receivemessage" :
			message, e := receiveMessage()
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

func sendMessage(message string) error {
	t := time.Now()
	svc := sqs.New(session.New(), &aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	})
	params := &sqs.SendMessageInput{
		MessageBody:            aws.String(message),
		QueueUrl:               aws.String(os.Getenv("QUEUE_URL")),
		MessageGroupId:         aws.String(os.Getenv("MESSAGE_GROUP_ID")),
		MessageDeduplicationId: aws.String(t.Format(layout2)),
	}
	_, err := svc.SendMessage(params)
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func getCount()(string, error) {
	svc := sqs.New(session.New(), &aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	})
	params := &sqs.GetQueueAttributesInput{
		AttributeNames: []*string{aws.String("ApproximateNumberOfMessages")},
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
	}
	res, err := svc.GetQueueAttributes(params)
	if err != nil {
		return "", err
	}
	return aws.StringValue(res.Attributes["ApproximateNumberOfMessages"]), nil
}

func deleteMessage(svc *sqs.SQS, msg *sqs.Message) error {
	params := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(os.Getenv("QUEUE_URL")),
		ReceiptHandle: aws.String(*msg.ReceiptHandle),
	}
	_, err := svc.DeleteMessage(params)

	if err != nil {
		return err
	}
	return nil
}

func receiveMessage()(string, error) {
	svc := sqs.New(session.New(), &aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	})
	params := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(os.Getenv("QUEUE_URL")),
		MaxNumberOfMessages: aws.Int64(1),
		WaitTimeSeconds: aws.Int64(3),
	}
	res, err := svc.ReceiveMessage(params)
	if err != nil {
		return "", err
	}
	if len(res.Messages) == 0 {
		return "Empty.", nil
	}
	var wg sync.WaitGroup
	for _, m := range res.Messages {
		wg.Add(1)
		go func(msg *sqs.Message) {
			defer wg.Done()
			if err := deleteMessage(svc, msg); err != nil {
				log.Println(err)
			}
		}(m)
	}
	wg.Wait()
	return aws.StringValue(res.Messages[0].Body), nil
}

func main() {
	lambda.Start(HandleRequest)
}
