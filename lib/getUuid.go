package lib

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
)

var xNode *snowflake.Node

func RegisterNodeID(nodeId int) error {
	xNodeId := int64(nodeId)
	node, err := snowflake.NewNode(xNodeId)
	if err != nil {
		return NewXError(err, fmt.Sprintf("注册节点ID失败: %v", xNodeId))
	}
	xNode = node
	logx.Infof("节点ID注册 %v\n", nodeId)
	return nil
}

func GetUuid() int64 {
	if xNode == nil {
		panic("请先注册节点ID")
	}
	return int64(xNode.Generate())
}
