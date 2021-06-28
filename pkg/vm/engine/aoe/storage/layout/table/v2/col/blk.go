package col

import (
	bmgrif "matrixone/pkg/vm/engine/aoe/storage/buffer/manager/iface"
	"matrixone/pkg/vm/engine/aoe/storage/common"
	"matrixone/pkg/vm/engine/aoe/storage/layout/base"
	"matrixone/pkg/vm/engine/aoe/storage/layout/index"
	"matrixone/pkg/vm/engine/aoe/storage/layout/table/v2/iface"
	md "matrixone/pkg/vm/engine/aoe/storage/metadata"
	"sync"
	"sync/atomic"
	// log "github.com/sirupsen/logrus"
)

type IColumnBlock interface {
	common.IRef
	// GetNext() IColumnBlock
	// SetNext(next IColumnBlock)
	GetID() uint64
	GetMeta() *md.Block
	GetRowCount() uint64
	// InitScanCursor(cusor *ScanCursor) error
	RegisterPart(part IColumnPart)
	// GetPartRoot() IColumnPart
	GetType() base.BlockType
	GetIndexHolder() *index.BlockHolder
	GetColIdx() int
	GetSegmentFile() base.ISegmentFile
	CloneWithUpgrade(iface.IBlock) IColumnBlock
	// EvalFilter(*index.FilterCtx) error
	String() string
	GetBlockHandle() iface.IColBlockHandle
}

type StdColBlockHandle struct {
	Node bmgrif.MangaedNode
}

func (h *StdColBlockHandle) Close() error {
	return h.Node.Close()
}

func (h *StdColBlockHandle) GetPageNode(pos int) bmgrif.MangaedNode {
	if pos > 0 {
		panic("logic error")
	}
	return h.Node
}

type ColumnBlock struct {
	sync.RWMutex
	common.RefHelper
	// Next        IColumnBlock
	ColIdx      int
	Meta        *md.Block
	SegmentFile base.ISegmentFile
	IndexHolder *index.BlockHolder
	Type        base.BlockType
}

// func (blk *ColumnBlock) EvalFilter(ctx *index.FilterCtx) error {
// 	return blk.IndexHolder.EvalFilter(blk.ColIdx, ctx)
// }

func (blk *ColumnBlock) GetSegmentFile() base.ISegmentFile {
	return blk.SegmentFile
}

func (blk *ColumnBlock) GetIndexHolder() *index.BlockHolder {
	return blk.IndexHolder
}

func (blk *ColumnBlock) GetColIdx() int {
	return blk.ColIdx
}

func (blk *ColumnBlock) GetMeta() *md.Block {
	return blk.Meta
}

func (blk *ColumnBlock) GetType() base.BlockType {
	return blk.Type
}

func (blk *ColumnBlock) GetRowCount() uint64 {
	return atomic.LoadUint64(&blk.Meta.Count)
}

// func (blk *ColumnBlock) SetNext(next IColumnBlock) {
// 	blk.Lock()
// 	defer blk.Unlock()
// 	if blk.Next != nil {
// 		blk.Next.UnRef()
// 	}
// 	blk.Next = next
// }

// func (blk *ColumnBlock) GetNext() IColumnBlock {
// 	blk.RLock()
// 	if blk.Next != nil {
// 		blk.Next.Ref()
// 	}
// 	r := blk.Next
// 	blk.RUnlock()
// 	return r
// }

func (blk *ColumnBlock) GetID() uint64 {
	return blk.Meta.ID
}