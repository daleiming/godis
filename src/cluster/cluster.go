package cluster

import (
    "fmt"
    "github.com/HDT3213/godis/src/config"
    "github.com/HDT3213/godis/src/db"
    "github.com/HDT3213/godis/src/interface/redis"
    "github.com/HDT3213/godis/src/lib/consistenthash"
    "github.com/HDT3213/godis/src/lib/logger"
    "github.com/HDT3213/godis/src/redis/client"
    "github.com/HDT3213/godis/src/redis/reply"
    "runtime/debug"
    "strconv"
)

type Cluster struct {
    bind string

    db         *db.DB
    peerPicker *consistenthash.Map
    peers      map[string]*client.Client
}

const (
    replicas = 4
)

func MakeCluster() *Cluster {
    cluster := &Cluster{
        bind: config.Properties.Bind + ":" + strconv.Itoa(config.Properties.Port),

        db:         db.MakeDB(),
        peerPicker: consistenthash.New(replicas, nil),
        peers:      make(map[string]*client.Client),
    }
    if config.Properties.Peers != nil && len(config.Properties.Peers) > 0 {
        cluster.peerPicker.Add(config.Properties.Peers...)
    }
    return cluster
}

func (cluster *Cluster) Close() {
    cluster.db.Close()
}

func (cluster *Cluster) Exec(c redis.Client, args [][]byte) (result redis.Reply) {
    defer func() {
        if err := recover(); err != nil {
            logger.Warn(fmt.Sprintf("error occurs: %v\n%s", err, string(debug.Stack())))
            result = &reply.UnknownErrReply{}
        }
    }()
    key := string(args[1])
    return cluster.Relay(key, c, args)
}

// relay command to peer
func (cluster *Cluster) Relay(key string, c redis.Client, args [][]byte) redis.Reply {
    peer := cluster.peerPicker.Get(key)
    if peer == cluster.bind {
        // to self db
        return cluster.db.Exec(c, args)
    } else {
        peerClient, ok := cluster.peers[peer]
        // lazy init
        if !ok {
            var err error
            peerClient, err = client.MakeClient(peer)
            if err != nil {
                return reply.MakeErrReply(err.Error())
            }
            peerClient.Start()
            cluster.peers[peer] = peerClient
        }
        return peerClient.Send(args)
    }
}

func (cluster *Cluster) AfterClientClose(c redis.Client) {

}