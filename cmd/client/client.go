package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"grpctest/proto"
	"io"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type client struct {
	chatClient proto.ChatClient
	scanner    *bufio.Scanner

	name string
	uuid string
}

func newClient(scanner *bufio.Scanner, name string) *client {
	return &client{
		scanner: scanner,
		name:    name,
	}
}

func (c *client) Run(host string) error {
	conn, err := grpc.Dial(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close()

	c.chatClient = proto.NewChatClient(conn)

	if err := c.setUUID(context.Background()); err != nil {
		return err
	}

	md := metadata.Pairs("uuid", c.uuid)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	stream, err := c.chatClient.Stream(ctx)
	if err != nil {
		return err
	}

	c.cliServe(stream)

	return nil
}

func (c *client) Close() error {
	_, err := c.chatClient.Close(context.Background(), &proto.CloseRequest{
		Uuid: c.uuid,
	})

	return err
}

func (c *client) setUUID(ctx context.Context) error {
	connectionResp, err := c.chatClient.Connect(context.Background(), &proto.ConnectRequest{
		Username: c.name,
	})
	if err != nil {
		return err
	}

	c.uuid = connectionResp.GetUuid()

	return nil
}

func (c *client) cliServe(stream proto.Chat_StreamClient) {
	fmt.Printf("\nHi, %s! Your UUID is %s.\n", c.name, c.uuid)
	fmt.Println("Now you can get list of all avaible commmands by typing /help.\n")

	for c.scanner.Scan() {
		argsSlice := strings.Split(strings.TrimSpace(c.scanner.Text()), " ")

		switch argsSlice[0] {
		case "/help":
			c.printHelpInformation()

		case "/all":
			if err := c.getClientsAndPrint(); err != nil {
				fmt.Println(err)
			}

		case "/join":
			if len(argsSlice) == 1 {
				fmt.Println("uuid is empty\n")
				continue
			}

			if err := c.joinToTheChat(stream, argsSlice[1]); err != nil {
				fmt.Println(err)
			}

		case "/quit":
			fmt.Println("You are not in chat with somebody.")

		default:
			fmt.Println("Invalid command, write /help to see avaible commands.\n")
		}

	}
}

func (c *client) printHelpInformation() {
	fmt.Println(" /all - that command will print you all avaible users.", "\n",
		"/join USERID - with that command you can join to the chat with user that was defined by USERID.", "\n",
		"/quit - that command helps you to quit from current chat.", "\n",
	)
}

func (c *client) getClientsAndPrint() error {
	clientsResp, err := c.chatClient.GetClients(context.Background(), &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("oops! It was unexpected error: %s", err)
	}

	if len(clientsResp.Clients) == 0 {
		return errors.New("nobody was connected to this server (even you)")
	}

	for uuid, clientOptions := range clientsResp.Clients {
		format := fmt.Sprintf("Name: %s, UUID: %s", uuid, clientOptions.GetName())
		if strings.EqualFold(uuid, c.uuid) {
			format = "(You) " + format
		}

		fmt.Println(format)
	}

	fmt.Println()

	return nil
}

func (c *client) joinToTheChat(stream proto.Chat_StreamClient, receiverUUID string) error {
	ctx, cancel := context.WithCancel(context.Background())
	if len(strings.TrimSpace(receiverUUID)) == 0 {
		return errors.New("receiverUUID is empty")
	}

	go c.sendMessageTo(ctx, cancel, stream, receiverUUID)

	go c.receiveMessage(ctx, cancel, stream)

mainLoop:
	for {
		select {
		case <-ctx.Done():
			break mainLoop
		}
	}

	return nil
}

func (c *client) sendMessageTo(ctx context.Context, cancel context.CancelFunc, stream proto.Chat_StreamClient, receiverUuid string) error {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-stream.Context().Done():
			return nil

		default:
			if c.scanner.Scan() {
				oldText := c.scanner.Text()
				text := strings.TrimSpace(oldText)

				switch text {
				case "/quit":
					fmt.Println("You are quiting from chat\n")

					return nil

				default:
					req := &proto.StreamRequest{
						ReceieverUuid: receiverUuid,
						Text:          oldText,
						Time:          timestamppb.Now(),
					}

					if err := stream.Send(req); err != nil {
						return err
					}

					fmt.Printf("Me, Text: %s, Time: %s\n", req.GetText(), req.GetTime())
				}
			} else {
				return errors.New("scanner input failure")
			}
		}
	}
}

func (c *client) receiveMessage(ctx context.Context, cancel context.CancelFunc, stream proto.Chat_StreamClient) error {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-stream.Context().Done():
			return nil

		default:
			resp, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return errors.New("stream was closed")
				}

				return err
			}

			fmt.Printf("UUID: %s, Text: %s, Time: %s\n", resp.GetReceieverUuid(), resp.GetText(), resp.GetTime())
		}
	}
}
