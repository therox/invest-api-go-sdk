package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/therox/invest-api-go-sdk/investgo"
	pb "github.com/therox/invest-api-go-sdk/proto"
	"go.uber.org/zap"
)

func main() {
	// Загружаем конфигурацию для сдк
	config, err := investgo.LoadConfig("config.yaml")
	if err != nil {
		log.Println("Cnf loading error", err.Error())
	}
	// контекст будет передан в сдк и будет использоваться для завершения работы
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Для примера передадим к качестве логгера uber zap
	prod, err := zap.NewProduction()
	defer func() {
		err := prod.Sync()
		if err != nil {
			log.Printf("Prod.Sync %v", err.Error())
		}
	}()

	if err != nil {
		log.Fatalf("logger creating error %e", err)
	}
	logger := prod.Sugar()

	// Создаем клиента для апи инвестиций, он поддерживает grpc соединение
	client, err := investgo.NewClient(ctx, config, logger)
	if err != nil {
		logger.Infof("Client creating error %v", err.Error())
	}
	defer func() {
		logger.Infof("Closing client connection")
		err := client.Stop()
		if err != nil {
			logger.Error("client shutdown error %v", err.Error())
		}
	}()
	// сервис песочницы нужен лишь для управления счетами песочнцы и пополнения баланса
	// остальной функционал доступен через обычные сервисы, но с эндпоинтом песочницы
	// для этого в конфиге сдк EndPoint = sandbox-invest-public-api.tinkoff.ru:443
	sandboxService := client.NewSandboxServiceClient()
	// открыть счет в песочнице можно через Kreya или BloomRPC, просто указав его в конфиге
	// или следующим образом из кода
	var newAccId string

	accountsResp, err := sandboxService.GetSandboxAccounts()
	if err != nil {
		logger.Errorf(err.Error())
	} else {
		accs := accountsResp.GetAccounts()
		if len(accs) > 0 {
			// если счета есть, берем первый
			newAccId = accs[0].GetId()
		} else {
			// если открытых счетов нет
			openAccount, err := sandboxService.OpenSandboxAccount()
			if err != nil {
				logger.Errorf(err.Error())
			} else {
				newAccId = openAccount.GetAccountId()
			}
			// запись в конфиг
			client.Config.AccountId = newAccId
		}
	}

	payInResp, err := sandboxService.SandboxPayIn(&investgo.SandboxPayInRequest{
		AccountId: newAccId,
		Currency:  "RUB",
		Unit:      100000,
		Nano:      0,
	})
	if err != nil {
		logger.Errorf(err.Error())
	} else {
		fmt.Printf("sandbox accouunt %v balance = %v\n", newAccId, payInResp.GetBalance().ToFloat())
	}
	// далее вызываем нужные нам сервисы, используя счет, токен, и эндпоинт песочницы
	instrumentsService := client.NewInstrumentsServiceClient()

	var id string
	instrumentResp, err := instrumentsService.FindInstrument("TCSG")
	if err != nil {
		logger.Errorf(err.Error())
	} else {
		instruments := instrumentResp.GetInstruments()
		for _, instrument := range instruments {
			if strings.Compare(instrument.GetTicker(), "TCSG") == 0 {
				id = instrument.GetUid()
			}
		}
	}
	ordersService := client.NewOrdersServiceClient()

	buyResp, err := ordersService.Buy(&investgo.PostOrderRequestShort{
		InstrumentId: id,
		Quantity:     1,
		Price:        nil,
		AccountId:    newAccId,
		OrderType:    pb.OrderType_ORDER_TYPE_MARKET,
		OrderId:      investgo.CreateUid(),
	})
	if err != nil {
		logger.Error(err.Error())
		fmt.Printf("msg = %v\n", investgo.MessageFromHeader(buyResp.GetHeader()))
	} else {
		fmt.Printf("order status = %v\n", buyResp.GetExecutionReportStatus().String())
	}

	operationsService := client.NewOperationsServiceClient()

	positionsResp, err := operationsService.GetPositions(newAccId)
	if err != nil {
		logger.Errorf(err.Error())
		fmt.Printf("msg = %v\n", investgo.MessageFromHeader(buyResp.GetHeader()))
	} else {
		positions := positionsResp.GetSecurities()
		for i, position := range positions {
			fmt.Printf("position number %v, uid = %v\n", i, position.GetInstrumentUid())
		}
	}

	sellResp, err := ordersService.Sell(&investgo.PostOrderRequestShort{
		InstrumentId: id,
		Quantity:     1,
		Price:        nil,
		AccountId:    newAccId,
		OrderType:    pb.OrderType_ORDER_TYPE_MARKET,
		OrderId:      investgo.CreateUid(),
	})
	if err != nil {
		logger.Error(err.Error())
	} else {
		fmt.Printf("order status = %v\n", sellResp.GetExecutionReportStatus().String())
	}

}
