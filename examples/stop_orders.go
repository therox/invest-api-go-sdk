package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/therox/invest-api-go-sdk/investgo"
	pb "github.com/therox/invest-api-go-sdk/proto"
	"go.uber.org/zap"
)

func main() {
	// Созадем клиента с grpc connection
	config, err := investgo.LoadConfig("config.yaml")
	if err != nil {
		log.Println("Cnf loading error", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prod, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("logger creating error %e", err)
	}
	logger := prod.Sugar()

	client, err := investgo.NewClient(ctx, config, logger)
	if err != nil {
		log.Printf("Client creating error %e", err)
	}
	defer func() {
		logger.Infof("Closing client connection")
		err := client.Stop()
		if err != nil {
			logger.Errorf(err.Error())
		}
	}()

	stopOrdersService := client.NewStopOrdersServiceClient()
	marketDataService := client.NewMarketDataServiceClient()

	var price *pb.Quotation

	lastPrice, err := marketDataService.GetLastPrices([]string{"BBG004S681W1"})
	if err != nil {
		logger.Error(err.Error())
	} else {
		prices := lastPrice.GetLastPrices()
		if len(prices) > 0 {
			price = prices[0].GetPrice()
		}
	}
	// Сервис стоп-ордеров работает только на продовом контуре, поэтому нужен токен полного доступа
	var orderId string
	stopOrderResp, err := stopOrdersService.PostStopOrder(&investgo.PostStopOrderRequest{
		InstrumentId: "BBG004S681W1",
		Quantity:     1,
		Price:        nil,
		StopPrice: &pb.Quotation{
			Units: price.Units + 1000,
			Nano:  price.Nano,
		},
		Direction:      pb.StopOrderDirection_STOP_ORDER_DIRECTION_SELL,
		AccountId:      config.AccountId,
		ExpirationType: pb.StopOrderExpirationType_STOP_ORDER_EXPIRATION_TYPE_GOOD_TILL_CANCEL,
		StopOrderType:  pb.StopOrderType_STOP_ORDER_TYPE_TAKE_PROFIT,
		ExpireDate:     time.Time{},
	})
	if err != nil {
		logger.Error(err.Error())
	} else {
		orderId = stopOrderResp.GetStopOrderId()
		fmt.Printf("Stop order id = %v\n", orderId)
	}

	cancelStopOrderResp, err := stopOrdersService.CancelStopOrder(config.AccountId, orderId)
	if err != nil {
		logger.Error(err.Error())
	} else {
		fmt.Printf("cancelling time = %v\n", cancelStopOrderResp.GetTime().AsTime().String())
	}
}
