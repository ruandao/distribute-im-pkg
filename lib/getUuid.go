package lib

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	"github.com/bwmarrin/snowflake"
)

var xNode *snowflake.Node

func RegisterNodeID(nodeId int) error {
	xNodeId := int64(nodeId)
	node, err := snowflake.NewNode(xNodeId)
	if err != nil {
		return xerr.NewXError(err, fmt.Sprintf("注册节点ID失败: %v", xNodeId))
	}
	xNode = node
	logx.Infof("节点ID注册 %v\n", nodeId)
	return nil
}

type Int64S int64

func (i Int64S) ToString() string {
	return strconv.FormatInt(int64(i), 10)
}
func (i Int64S) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(i), 10))
}

func GetUuid() Int64S {
	if xNode == nil {
		panic("请先注册节点ID")
	}
	return Int64S(xNode.Generate())
}
