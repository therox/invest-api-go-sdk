package investgo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	pb "github.com/therox/invest-api-go-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type MarketDataServiceClient struct {
	conn     *grpc.ClientConn
	config   Config
	logger   Logger
	ctx      context.Context
	pbClient pb.MarketDataServiceClient
}

// GetCandles - Метод запроса исторических свечей по инструменту
func (md *MarketDataServiceClient) GetCandles(instrumentId string, interval pb.CandleInterval, from, to time.Time) (*GetCandlesResponse, error) {
	var header, trailer metadata.MD
	resp, err := md.pbClient.GetCandles(md.ctx, &pb.GetCandlesRequest{
		From:         TimeToTimestamp(from),
		To:           TimeToTimestamp(to),
		Interval:     interval,
		InstrumentId: instrumentId,
	}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		header = trailer
	}
	return &GetCandlesResponse{
		GetCandlesResponse: resp,
		Header:             header,
	}, err
}

// GetLastPrices - Метод запроса цен последних сделок по инструментам
func (md *MarketDataServiceClient) GetLastPrices(instrumentIds []string) (*GetLastPricesResponse, error) {
	var header, trailer metadata.MD
	resp, err := md.pbClient.GetLastPrices(md.ctx, &pb.GetLastPricesRequest{
		InstrumentId: instrumentIds,
	}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		header = trailer
	}
	return &GetLastPricesResponse{
		GetLastPricesResponse: resp,
		Header:                header,
	}, err
}

// GetOrderBook - Метод получения стакана по инструменту
func (md *MarketDataServiceClient) GetOrderBook(instrumentId string, depth int32) (*GetOrderBookResponse, error) {
	var header, trailer metadata.MD
	resp, err := md.pbClient.GetOrderBook(md.ctx, &pb.GetOrderBookRequest{
		Depth:        depth,
		InstrumentId: instrumentId,
	}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		header = trailer
	}
	return &GetOrderBookResponse{
		GetOrderBookResponse: resp,
		Header:               header,
	}, err
}

// GetTradingStatus - Метод запроса статуса торгов по инструменту
func (md *MarketDataServiceClient) GetTradingStatus(instrumentId string) (*GetTradingStatusResponse, error) {
	var header, trailer metadata.MD
	resp, err := md.pbClient.GetTradingStatus(md.ctx, &pb.GetTradingStatusRequest{
		InstrumentId: instrumentId,
	}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		header = trailer
	}
	return &GetTradingStatusResponse{
		GetTradingStatusResponse: resp,
		Header:                   header,
	}, err
}

// GetTradingStatuses - Метод запроса статуса торгов по инструментам
func (md *MarketDataServiceClient) GetTradingStatuses(instrumentIds []string) (*GetTradingStatusesResponse, error) {
	var header, trailer metadata.MD
	resp, err := md.pbClient.GetTradingStatuses(md.ctx, &pb.GetTradingStatusesRequest{
		InstrumentId: instrumentIds,
	}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		header = trailer
	}
	return &GetTradingStatusesResponse{
		GetTradingStatusesResponse: resp,
		Header:                     header,
	}, err
}

// GetLastTrades - Метод запроса обезличенных сделок за последний час
func (md *MarketDataServiceClient) GetLastTrades(instrumentId string, from, to time.Time) (*GetLastTradesResponse, error) {
	var header, trailer metadata.MD
	resp, err := md.pbClient.GetLastTrades(md.ctx, &pb.GetLastTradesRequest{
		From:         TimeToTimestamp(from),
		To:           TimeToTimestamp(to),
		InstrumentId: instrumentId,
	}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		header = trailer
	}
	return &GetLastTradesResponse{
		GetLastTradesResponse: resp,
		Header:                header,
	}, err
}

// GetClosePrices - Метод запроса цен закрытия торговой сессии по инструментам
func (md *MarketDataServiceClient) GetClosePrices(instrumentIds []string) (*GetClosePricesResponse, error) {
	var header, trailer metadata.MD
	instruments := make([]*pb.InstrumentClosePriceRequest, 0, len(instrumentIds))
	for _, id := range instrumentIds {
		instruments = append(instruments, &pb.InstrumentClosePriceRequest{InstrumentId: id})
	}
	resp, err := md.pbClient.GetClosePrices(md.ctx, &pb.GetClosePricesRequest{
		Instruments: instruments,
	}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		header = trailer
	}
	return &GetClosePricesResponse{
		GetClosePricesResponse: resp,
		Header:                 header,
	}, err
}

// GetHistoricCandles - Метод загрузки исторических свечей.
// Если указать File = true, то создастся .csv файл с записями
// свечей в формате: instrumentId;time;open;close;high;low;volume.
// Имя файла по умолчанию: "candles hh:mm:ss"
func (md *MarketDataServiceClient) GetHistoricCandles(req *GetHistoricCandlesRequest) ([]*pb.HistoricCandle, error) {
	duration := selectDuration(req.Interval)
	// если запрашиваемый интервал больше чем возможный, то нужно разделить его на несколько
	intervals := make([]time.Time, 0)
	if req.To.Sub(req.From) > duration {
		lowTime := req.To
		for lowTime.After(req.From) || lowTime.Equal(req.From) {
			intervals = append(intervals, lowTime)
			lowTime = lowTime.Add(-duration)
		}
		intervals = append(intervals, req.From)
	} else {
		intervals = []time.Time{req.From, req.To}
	}
	// intervals = {to, ... , from}

	candles := make([]*pb.HistoricCandle, 0)
	requests := 0
	for i := len(intervals) - 1; i > 0; i-- {
		// идем с конца слайса так как там более раннее время
		// from - i элемент
		// to - i-1 элемент
		requests++
		resp, err := md.GetCandles(req.Instrument, req.Interval, intervals[i], intervals[i-1])
		if err != nil {
			return nil, err
		}
		candles = append(candles, resp.GetCandles()...)
		if requests == 299 {
			time.Sleep(time.Minute)
			requests = 0
		}
	}

	if req.File {
		err := md.writeCandlesToFile(candles, req.Instrument, req.FileName)
		if err != nil {
			return candles, err
		}
	}

	return candles, nil
}

// GetAllHistoricCandles - Метод получения всех свечей по инструменту, поля from, to игнорируются
func (md *MarketDataServiceClient) GetAllHistoricCandles(req *GetHistoricCandlesRequest) ([]*pb.HistoricCandle, error) {
	instrumentsService := &InstrumentsServiceClient{
		conn:     md.conn,
		config:   md.config,
		logger:   md.logger,
		ctx:      md.ctx,
		pbClient: pb.NewInstrumentsServiceClient(md.conn),
	}

	resp, err := instrumentsService.FindInstrument(req.Instrument)
	if err != nil {
		return nil, err
	}
	ids := resp.GetInstruments()
	if len(ids) < 1 {
		return nil, fmt.Errorf("Instrument %v not found\n", req.Instrument)
	}

	var from time.Time
	switch req.Interval {
	case pb.CandleInterval_CANDLE_INTERVAL_DAY, pb.CandleInterval_CANDLE_INTERVAL_WEEK, pb.CandleInterval_CANDLE_INTERVAL_MONTH:
		from = resp.GetInstruments()[0].GetFirst_1DayCandleDate().AsTime()
	default:
		from = resp.GetInstruments()[0].GetFirst_1MinCandleDate().AsTime()
	}

	return md.GetHistoricCandles(&GetHistoricCandlesRequest{
		Instrument: req.Instrument,
		Interval:   req.Interval,
		From:       from,
		To:         time.Now(),
		File:       req.File,
		FileName:   req.FileName,
	})
}

// by default 1 hour
func selectDuration(interval pb.CandleInterval) time.Duration {
	var duration time.Duration
	switch interval {
	case pb.CandleInterval_CANDLE_INTERVAL_1_MIN, pb.CandleInterval_CANDLE_INTERVAL_2_MIN, pb.CandleInterval_CANDLE_INTERVAL_3_MIN:
		duration = time.Hour * 24
	case pb.CandleInterval_CANDLE_INTERVAL_5_MIN, pb.CandleInterval_CANDLE_INTERVAL_10_MIN, pb.CandleInterval_CANDLE_INTERVAL_15_MIN:
		duration = time.Hour * 24
	case pb.CandleInterval_CANDLE_INTERVAL_30_MIN:
		duration = time.Hour * 48
	case pb.CandleInterval_CANDLE_INTERVAL_HOUR:
		duration = time.Hour * 24 * 7
	case pb.CandleInterval_CANDLE_INTERVAL_2_HOUR, pb.CandleInterval_CANDLE_INTERVAL_4_HOUR:
		duration = time.Hour * 24 * 30
	case pb.CandleInterval_CANDLE_INTERVAL_DAY:
		duration = time.Hour * 24 * 360
	case pb.CandleInterval_CANDLE_INTERVAL_WEEK:
		duration = time.Hour * 24 * 360 * 2
	case pb.CandleInterval_CANDLE_INTERVAL_MONTH:
		duration = time.Hour * 24 * 360 * 2
	case pb.CandleInterval_CANDLE_INTERVAL_UNSPECIFIED:
		duration = time.Hour * 24 * 7
		interval = pb.CandleInterval_CANDLE_INTERVAL_HOUR
	}
	return duration
}

// Метод записи в .csv файл исторических свечей в формате instrumentId;time;open;close;high;low;volume
func (md *MarketDataServiceClient) writeCandlesToFile(candles []*pb.HistoricCandle, id string, filename string) error {
	h, m, s := time.Now().Clock()
	if strings.Compare(filename, "") == 0 {
		filename = fmt.Sprintf("candles %v:%v:%v", h, m, s)
	}

	file, err := os.Create(fmt.Sprintf("%v.csv", filename))
	if err != nil {
		return err
	}
	defer file.Close()
	for _, candle := range candles {
		fmt.Fprintf(file, "%v;%v\n", id, candle.ToCSV())
	}
	return nil
}
