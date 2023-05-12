package spdk

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	spdkclient "github.com/longhorn/go-spdk-helper/pkg/spdk/client"
	spdktypes "github.com/longhorn/go-spdk-helper/pkg/spdk/types"

	"github.com/longhorn/longhorn-spdk-engine/pkg/util"
	"github.com/longhorn/longhorn-spdk-engine/proto/spdkrpc"
)

const (
	MonitorInterval = 3 * time.Second
)

type Server struct {
	sync.RWMutex

	ctx context.Context

	spdkClient    *spdkclient.Client
	portAllocator *util.Bitmap

	replicaMap map[string]*Replica
}

func NewServer(ctx context.Context, portStart, portEnd int32) (*Server, error) {
	cli, err := spdkclient.NewClient()
	if err != nil {
		return nil, err
	}

	s := &Server{
		ctx: ctx,

		spdkClient:    cli,
		portAllocator: util.NewBitmap(portStart, portEnd),

		replicaMap: map[string]*Replica{},
	}

	// TODO: There is no need to maintain the replica map in cache when we can use one SPDK JSON API call to fetch the Lvol tree/chain info
	go s.monitoringReplicas()

	return s, nil
}

func (s *Server) monitoringReplicas() {
	ticker := time.NewTicker(MonitorInterval)
	defer ticker.Stop()

	done := false
	for {
		select {
		case <-s.ctx.Done():
			logrus.Info("SPDK Server: stopped monitoring replicas due to the context done")
			done = true
		case <-ticker.C:
			bdevLvolList, err := s.spdkClient.BdevLvolGet("", 0)
			if err != nil {
				logrus.Errorf("SPDK Server: failed to get lvol bdevs for replica cache update, will retry later: %v", err)
				continue
			}
			bdevLvolMap := map[string]*spdktypes.BdevInfo{}
			for idx := range bdevLvolList {
				bdevLvol := &bdevLvolList[idx]
				if len(bdevLvol.Aliases) == 0 {
					continue
				}
				bdevLvolMap[spdktypes.GetLvolNameFromAlias(bdevLvol.Aliases[0])] = bdevLvol
			}

			s.Lock()
			for _, r := range s.replicaMap {
				r.ValidateAndUpdate(bdevLvolMap)
			}
			s.Unlock()
		}
		if done {
			break
		}
	}
}

func (s *Server) ReplicaCreate(ctx context.Context, req *spdkrpc.ReplicaCreateRequest) (ret *spdkrpc.Replica, err error) {
	s.Lock()
	if s.replicaMap[req.Name] == nil {
		s.replicaMap[req.Name] = NewReplica(req.Name, req.LvsName, req.LvsUuid, req.SpecSize)
	}
	r := s.replicaMap[req.Name]
	s.Unlock()

	portStart, _, err := s.portAllocator.AllocateRange(1)
	if err != nil {
		return nil, err
	}

	return r.Create(s.spdkClient, portStart, req.ExposeRequired)
}

func (s *Server) ReplicaDelete(ctx context.Context, req *spdkrpc.ReplicaDeleteRequest) (ret *empty.Empty, err error) {
	s.RLock()
	r := s.replicaMap[req.Name]
	delete(s.replicaMap, req.Name)
	s.Unlock()

	if r != nil {
		if err := r.Delete(s.spdkClient, req.CleanupRequired); err != nil {
			return nil, err
		}
	}

	return &empty.Empty{}, nil
}

func (s *Server) ReplicaGet(ctx context.Context, req *spdkrpc.ReplicaGetRequest) (ret *spdkrpc.Replica, err error) {
	s.RLock()
	r := s.replicaMap[req.Name]
	s.RUnlock()

	if r == nil {
		return nil, fmt.Errorf("replica %s is not found during get", req.Name)
	}

	return r.Get()
}

func (s *Server) ReplicaList(ctx context.Context, req *empty.Empty) (*spdkrpc.ReplicaListResponse, error) {
	// TODO: Implement this
	return &spdkrpc.ReplicaListResponse{}, nil
}

func (s *Server) ReplicaWatch(req *empty.Empty, srv spdkrpc.SPDKService_ReplicaWatchServer) error {
	// TODO: Implement this
	return nil
}

func (s *Server) ReplicaSnapshotCreate(ctx context.Context, req *spdkrpc.SnapshotRequest) (ret *spdkrpc.Replica, err error) {
	s.RLock()
	r := s.replicaMap[req.Name]
	s.RUnlock()

	if r == nil {
		return nil, fmt.Errorf("replica %s is not found during snapshot create", req.Name)
	}

	return r.SnapshotCreate(s.spdkClient, req.Name)
}

func (s *Server) ReplicaSnapshotDelete(ctx context.Context, req *spdkrpc.SnapshotRequest) (ret *empty.Empty, err error) {
	s.RLock()
	r := s.replicaMap[req.Name]
	s.RUnlock()

	if r == nil {
		return nil, fmt.Errorf("replica %s is not found during snapshot delete", req.Name)
	}

	_, err = r.SnapshotDelete(s.spdkClient, req.Name)
	return &empty.Empty{}, err
}

func (s *Server) EngineCreate(ctx context.Context, req *spdkrpc.EngineCreateRequest) (ret *spdkrpc.Engine, err error) {
	portStart, _, err := s.portAllocator.AllocateRange(1)
	if err != nil {
		return nil, err
	}

	return SvcEngineCreate(s.spdkClient, req.Name, req.Frontend, req.ReplicaAddressMap, portStart)
}

func (s *Server) EngineDelete(ctx context.Context, req *spdkrpc.EngineDeleteRequest) (ret *empty.Empty, err error) {
	if err = SvcEngineDelete(s.spdkClient, req.Name); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *Server) EngineGet(ctx context.Context, req *spdkrpc.EngineGetRequest) (ret *spdkrpc.Engine, err error) {
	return SvcEngineGet(s.spdkClient, req.Name)
}

func (s *Server) EngineList(ctx context.Context, req *empty.Empty) (*spdkrpc.EngineListResponse, error) {
	// TODO: Implement this
	return &spdkrpc.EngineListResponse{}, nil
}

func (s *Server) EngineWatch(req *empty.Empty, srv spdkrpc.SPDKService_EngineWatchServer) error {
	// TODO: Implement this
	return nil
}

func (s *Server) EngineSnapshotCreate(ctx context.Context, req *spdkrpc.SnapshotRequest) (ret *spdkrpc.Engine, err error) {
	return SvcEngineSnapshotCreate(s.spdkClient, req.Name, req.SnapshotName)
}

func (s *Server) EngineSnapshotDelete(ctx context.Context, req *spdkrpc.SnapshotRequest) (ret *empty.Empty, err error) {
	_, err = SvcEngineSnapshotDelete(s.spdkClient, req.Name, req.SnapshotName)
	return &empty.Empty{}, err
}

func (s *Server) VersionDetailGet(context.Context, *empty.Empty) (*spdkrpc.VersionDetailGetReply, error) {
	// TODO: Implement this
	return &spdkrpc.VersionDetailGetReply{
		Version: &spdkrpc.VersionOutput{},
	}, nil
}
