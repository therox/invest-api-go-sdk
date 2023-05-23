package investgo

import (
	"context"

	pb "github.com/therox/invest-api-go-sdk/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PortfolioStream struct {
	stream           pb.OperationsStreamService_PortfolioStreamClient
	operationsClient *OperationsStreamClient

	ctx    context.Context
	cancel context.CancelFunc

	portfolios chan *pb.PortfolioResponse
}

// Portfolios - Метод возвращает канал для чтения обновлений портфеля
func (p *PortfolioStream) Portfolios() <-chan *pb.PortfolioResponse {
	return p.portfolios
}

// Listen - метод начинает слушать стрим и отправлять информацию в канал, для получения канала: Portfolios()
func (p *PortfolioStream) Listen() error {
	defer p.shutdown()
	for {
		select {
		case <-p.ctx.Done():
			return nil
		default:
			resp, err := p.stream.Recv()
			if err != nil {
				switch {
				case status.Code(err) == codes.Canceled:
					p.operationsClient.logger.Infof("Stop listening portfolios")
					return nil
				default:
					return err
				}
			} else {
				switch resp.GetPayload().(type) {
				case *pb.PortfolioStreamResponse_Portfolio:
					p.portfolios <- resp.GetPortfolio()
				default:
					p.operationsClient.logger.Infof("Info from Portfolio stream %v", resp.String())
				}
			}
		}
	}
}

func (p *PortfolioStream) shutdown() {
	p.operationsClient.logger.Infof("Close portfolio stream")
	close(p.portfolios)
}

// Stop - Завершение работы стрима
func (p *PortfolioStream) Stop() {
	p.cancel()
}
