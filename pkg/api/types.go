package api

import (
	"github.com/longhorn/longhorn-spdk-engine/pkg/types"
	"github.com/longhorn/longhorn-spdk-engine/proto/ptypes"
)

type Replica struct {
	Name       string           `json:"name"`
	UUID       string           `json:"uuid"`
	LvsName    string           `json:"lvs_name"`
	LvsUUID    string           `json:"lvs_uuid"`
	SpecSize   uint64           `json:"spec_size"`
	ActualSize uint64           `json:"actual_size"`
	Snapshots  map[string]*Lvol `json:"snapshots"`
}

type Lvol struct {
	Name       string          `json:"name"`
	UUID       string          `json:"uuid"`
	SpecSize   uint64          `json:"spec_size"`
	ActualSize uint64          `json:"actual_size"`
	Parent     string          `json:"parent"`
	Children   map[string]bool `json:"children"`
}

func ProtoLvolToLvol(l *ptypes.Lvol) *Lvol {
	return &Lvol{
		Name:       l.Name,
		UUID:       l.Uuid,
		SpecSize:   l.SpecSize,
		ActualSize: l.ActualSize,
		Parent:     l.Parent,
		Children:   l.Children,
	}
}
func ProtoReplicaToReplica(r *ptypes.Replica) *Replica {
	res := &Replica{
		Name:       r.Name,
		UUID:       r.Uuid,
		LvsName:    r.LvsName,
		LvsUUID:    r.LvsUuid,
		SpecSize:   r.SpecSize,
		ActualSize: r.ActualSize,
		Snapshots:  map[string]*Lvol{},
	}
	for snapName, snapProtoLvol := range r.Snapshots {
		res.Snapshots[snapName] = ProtoLvolToLvol(snapProtoLvol)
	}

	return res
}

type Engine struct {
	Name              string                `json:"name"`
	UUID              string                `json:"uuid"`
	SpecSize          uint64                `json:"spec_size"`
	ActualSize        uint64                `json:"actual_size"`
	IP                string                `json:"ip"`
	Port              int32                 `json:"port"`
	ReplicaAddressMap map[string]string     `json:"replica_address_map"`
	ReplicaModeMap    map[string]types.Mode `json:"replica_mode_map"`
	Endpoint          string                `json:"endpoint"`
}

func ProtoEngineToEngine(e *ptypes.Engine) *Engine {
	res := &Engine{
		Name:              e.Name,
		UUID:              e.Uuid,
		SpecSize:          e.SpecSize,
		ActualSize:        e.ActualSize,
		IP:                e.Ip,
		Port:              e.Port,
		ReplicaAddressMap: e.ReplicaAddressMap,
		ReplicaModeMap:    map[string]types.Mode{},
		Endpoint:          e.Endpoint,
	}
	for rName, mode := range e.ReplicaModeMap {
		res.ReplicaModeMap[rName] = ptypes.GRPCReplicaModeToReplicaMode(mode)
	}

	return res
}
