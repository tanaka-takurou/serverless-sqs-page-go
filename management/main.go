package main

import (
	"log"
	"flag"
	"sync"
	"time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

var svc *sqs.SQS

const (
	LAYOUT = "20060102150405.000"
	AWS_REGION = "ap-northeast-1"
	MESSAGE_GROUP_ID = "YourMessageGroupId"
	QUEUE_URL  = "YourQueueUrl"
)

func SendMessage() error {
	t := time.Now()
	params := &sqs.SendMessageInput{
		MessageBody:            aws.String("Message:" + t.Format(LAYOUT)),
		QueueUrl:               aws.String(QUEUE_URL),
		MessageGroupId:         aws.String(MESSAGE_GROUP_ID),
		MessageDeduplicationId: aws.String(t.Format(LAYOUT)),
	}

	res, err := svc.SendMessage(params)
	if err != nil {
		return err
	}

	log.Println("SQSMessageID", *res.MessageId)

	return nil
}

func GetMessageCount() error {
	params := &sqs.GetQueueAttributesInput{
		AttributeNames: []*string{aws.String("ApproximateNumberOfMessages")},
		QueueUrl: aws.String(QUEUE_URL),
	}
	res, err := svc.GetQueueAttributes(params)

	if err != nil {
		return err
	}

	for k, v := range res.Attributes {
		log.Printf("%v: %v", k, aws.StringValue(v))
	}
	return nil
}

func ReceiveMessage() error {
	params := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(QUEUE_URL),
		MaxNumberOfMessages: aws.Int64(1),
		WaitTimeSeconds: aws.Int64(3),
	}
	res, err := svc.ReceiveMessage(params)

	if err != nil {
		return err
	}

	log.Printf("Messages count: %d\n", len(res.Messages))

	if len(res.Messages) == 0 {
		log.Println("Empty queue.")
		return nil
	}

	var wg sync.WaitGroup
	for _, m := range res.Messages {
		wg.Add(1)
		go func(msg *sqs.Message) {
			defer wg.Done()
			log.Println(aws.StringValue(msg.Body))
			if err := DeleteMessage(msg); err != nil {
				log.Println(err)
			}
		}(m)
	}

	wg.Wait()

	return nil
}

func DeleteMessage(msg *sqs.Message) error {
	params := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(QUEUE_URL),
		ReceiptHandle: aws.String(*msg.ReceiptHandle),
	}
	_, err := svc.DeleteMessage(params)

	if err != nil {
		return err
	}
	return nil
}

func main() {
	log.Println("[ SQS Management ]")
	flag.Parse()
	svc = sqs.New(session.New(), &aws.Config{
		Region: aws.String("ap-northeast-1"),
	})

	switch flag.Arg(0) {
	case "send":
		if err := SendMessage(); err != nil {
			log.Fatal(err)
		}
	case "count":
		if err := GetMessageCount(); err != nil {
			log.Fatal(err)
		}
	case "receive":
		if err := ReceiveMessage(); err != nil {
			log.Fatal(err)
		}
	default:
		log.Println("Error: Bad Command. [send, count, receive]")
	}
}
