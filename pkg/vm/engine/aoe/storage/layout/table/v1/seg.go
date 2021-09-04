package table

import (
	"fmt"
	logutil2 "matrixone/pkg/logutil"
	bmgrif "matrixone/pkg/vm/engine/aoe/storage/buffer/manager/iface"
	"matrixone/pkg/vm/engine/aoe/storage/common"
	"matrixone/pkg/vm/engine/aoe/storage/layout/base"
	"matrixone/pkg/vm/engine/aoe/storage/layout/index"
	"matrixone/pkg/vm/engine/aoe/storage/layout/table/v1/iface"
	md "matrixone/pkg/vm/engine/aoe/storage/metadata/v1"
	"sync"
	"sync/atomic"
)

type Segment struct {
	common.SLLNode
	Type base.SegmentType
	tree struct {
		*sync.RWMutex
		Blocks   []iface.IBlock
		Helper   map[uint64]int
		BlockIds []uint64
		BlockCnt uint32
		AttrSize map[string]uint64
	}
	MTBufMgr    bmgrif.IBufferManager
	SSTBufMgr   bmgrif.IBufferManager
	Meta        *md.Segment
	IndexHolder *index.SegmentHolder
	FsMgr       base.IManager
	SegmentFile base.ISegmentFile
}

func NewSegment(host iface.ITableData, meta *md.Segment) (iface.ISegment, error) {
	var err error
	segType := base.UNSORTED_SEG
	if meta.DataState == md.SORTED {
		segType = base.SORTED_SEG
	}
	mu := new(sync.RWMutex)
	seg := &Segment{
		Type:      segType,
		MTBufMgr:  host.GetMTBufMgr(),
		SSTBufMgr: host.GetSSTBufMgr(),
		FsMgr:     host.GetFsManager(),
		Meta:      meta,
		SLLNode:   *common.NewSLLNode(mu),
	}

	segId := meta.AsCommonID().AsSegmentID()
	indexHolder := host.GetIndexHolder().RegisterSegment(segId, segType, nil)
	seg.IndexHolder = indexHolder
	segFile := seg.FsMgr.GetUnsortedFile(segId)
	if segType == base.UNSORTED_SEG {
		if segFile == nil {
			segFile, err = seg.FsMgr.RegisterUnsortedFiles(segId)
			if err != nil {
				panic(err)
			}
		}
		seg.IndexHolder.Init(segFile)
	} else {
		if segFile != nil {
			seg.FsMgr.UpgradeFile(segId)
		} else {
			segFile = seg.FsMgr.GetSortedFile(segId)
			if segFile == nil {
				segFile, err = seg.FsMgr.RegisterSortedFiles(segId)
				if err != nil {
					panic(err)
				}
			}
		}
		seg.IndexHolder.Init(segFile)
	}

	seg.tree.RWMutex = mu
	seg.tree.Blocks = make([]iface.IBlock, 0)
	seg.tree.Helper = make(map[uint64]int)
	seg.tree.BlockIds = make([]uint64, 0)
	seg.tree.AttrSize = make(map[string]uint64)
	seg.OnZeroCB = seg.close
	seg.SegmentFile = segFile
	seg.SegmentFile.Ref()
	seg.Ref()
	return seg, nil
}

func (seg *Segment) CanUpgrade() bool {
	if seg.Type == base.SORTED_SEG {
		return false
	}
	if len(seg.tree.Blocks) < int(seg.Meta.Table.Conf.SegmentMaxBlocks) {
		return false
	}
	for _, blk := range seg.tree.Blocks {
		if blk.GetType() != base.PERSISTENT_BLK {
			return false
		}
	}
	return true
}

func (seg *Segment) GetSegmentedIndex() (id uint64, ok bool) {
	ok = false
	if seg.Type == base.SORTED_SEG {
		for i := len(seg.tree.Blocks) - 1; i >= 0; i-- {
			id, ok = seg.tree.Blocks[i].GetSegmentedIndex()
			if ok {
				return id, ok
			}
		}
		return id, ok
	}
	blkCnt := atomic.LoadUint32(&seg.tree.BlockCnt)
	for i := int(blkCnt) - 1; i >= 0; i-- {
		seg.tree.RLock()
		blk := seg.tree.Blocks[i]
		seg.tree.RUnlock()
		id, ok = blk.GetSegmentedIndex()
		if ok {
			return id, ok
		}
	}
	return id, ok
}

func (seg *Segment) GetReplayIndex() *md.LogIndex {
	if seg.tree.BlockCnt == 0 {
		return nil
	}
	var ctx *md.LogIndex
	for blkIdx := int(seg.tree.BlockCnt) - 1; blkIdx >= 0; blkIdx-- {
		blk := seg.tree.Blocks[blkIdx]
		if ctx = blk.GetMeta().GetReplayIndex(); ctx != nil {
			break
		}
	}
	return ctx
}

func (seg *Segment) GetRowCount() uint64 {
	if seg.Meta.DataState >= md.CLOSED {
		return seg.Meta.Table.Conf.BlockMaxRows * seg.Meta.Table.Conf.SegmentMaxBlocks
	}
	var ret uint64
	seg.tree.RLock()
	for _, blk := range seg.tree.Blocks {
		ret += blk.GetRowCount()
	}
	seg.tree.RUnlock()
	return ret
}

func (seg *Segment) Size(attr string) uint64 {
	if seg.Type >= base.SORTED_SEG {
		return seg.tree.AttrSize[attr]
	}
	size := uint64(0)
	blkCnt := atomic.LoadUint32(&seg.tree.BlockCnt)
	var blk iface.IBlock
	for i := 0; i < int(blkCnt); i++ {
		seg.tree.RLock()
		blk = seg.tree.Blocks[i]
		seg.tree.RUnlock()
		size += blk.Size(attr)
	}
	return size
}

func (seg *Segment) BlockIds() []uint64 {
	if seg.Type == base.SORTED_SEG {
		return seg.tree.BlockIds
	}
	if atomic.LoadUint32(&seg.tree.BlockCnt) == uint32(seg.Meta.Table.Conf.SegmentMaxBlocks) {
		return seg.tree.BlockIds
	}
	seg.tree.RLock()
	ret := make([]uint64, 0, atomic.LoadUint32(&seg.tree.BlockCnt))
	for _, blk := range seg.tree.Blocks {
		ret = append(ret, blk.GetMeta().ID)
	}
	seg.tree.RUnlock()
	return ret
}

func (seg *Segment) close() {
	segId := seg.Meta.AsCommonID().AsSegmentID()
	if seg.IndexHolder != nil {
		seg.IndexHolder.Unref()
	}
	for _, blk := range seg.tree.Blocks {
		blk.Unref()
		// log.Infof("blk refs=%d", blk.RefCount())
	}
	seg.SLLNode.ReleaseNextNode()

	if seg.SegmentFile != nil {
		seg.SegmentFile.Unref()
	}
	if seg.Type == base.UNSORTED_SEG {
		seg.FsMgr.UnregisterUnsortedFile(segId)
	} else {
		seg.FsMgr.UnregisterSortedFile(segId)
	}
}

func (seg *Segment) SetNext(next iface.ISegment) {
	seg.SLLNode.SetNextNode(next)
}

func (seg *Segment) GetNext() iface.ISegment {
	r := seg.SLLNode.GetNextNode()
	if r == nil {
		return nil
	}
	return r.(iface.ISegment)
}

func (seg *Segment) GetMeta() *md.Segment {
	return seg.Meta
}

func (seg *Segment) GetSegmentFile() base.ISegmentFile {
	return seg.SegmentFile
}

func (seg *Segment) GetType() base.SegmentType {
	return seg.Type
}

func (seg *Segment) GetMTBufMgr() bmgrif.IBufferManager {
	return seg.MTBufMgr
}

func (seg *Segment) GetSSTBufMgr() bmgrif.IBufferManager {
	return seg.SSTBufMgr
}

func (seg *Segment) GetFsManager() base.IManager {
	return seg.FsMgr
}

func (seg *Segment) GetIndexHolder() *index.SegmentHolder {
	return seg.IndexHolder
}

func (seg *Segment) String() string {
	seg.tree.RLock()
	defer seg.tree.RUnlock()
	s := fmt.Sprintf("<Segment[%d]>(BlkCnt=%d)(Refs=%d)(IndexRefs=%d)", seg.Meta.ID, seg.tree.BlockCnt, seg.RefCount(), seg.IndexHolder.RefCount())
	for _, blk := range seg.tree.Blocks {
		s = fmt.Sprintf("%s\n\t%s", s, blk.String())
		prev := blk.GetPrevVersion()
		v := 0
		for prev != nil {
			s = fmt.Sprintf("%s V%d", s, v)
			v++
			prev = prev.(*Block).GetPrevVersion()
		}
		s = fmt.Sprintf("%s V%d", s, v)
	}
	return s
}

func (seg *Segment) RegisterBlock(blkMeta *md.Block) (blk iface.IBlock, err error) {
	blk, err = NewBlock(seg, blkMeta)
	if err != nil {
		return nil, err
	}
	seg.tree.Lock()
	defer seg.tree.Unlock()
	if len(seg.tree.Blocks) > 0 {
		blk.Ref()
		seg.tree.Blocks[len(seg.tree.Blocks)-1].SetNext(blk)
	}

	seg.tree.Blocks = append(seg.tree.Blocks, blk)
	seg.tree.BlockIds = append(seg.tree.BlockIds, blk.GetMeta().ID)
	seg.tree.Helper[blkMeta.ID] = int(seg.tree.BlockCnt)
	atomic.AddUint32(&seg.tree.BlockCnt, uint32(1))
	blk.Ref()
	return blk, err
}

func (seg *Segment) WeakRefBlock(id uint64) iface.IBlock {
	seg.tree.RLock()
	defer seg.tree.RUnlock()
	idx, ok := seg.tree.Helper[id]
	if !ok {
		return nil
	}
	return seg.tree.Blocks[idx]
}

func (seg *Segment) StrongRefBlock(id uint64) iface.IBlock {
	seg.tree.RLock()
	defer seg.tree.RUnlock()
	idx, ok := seg.tree.Helper[id]
	if !ok {
		return nil
	}
	blk := seg.tree.Blocks[idx]
	blk.Ref()
	return blk
}
func (seg *Segment) CloneWithUpgrade(td iface.ITableData, meta *md.Segment) (iface.ISegment, error) {
	if seg.Type != base.UNSORTED_SEG {
		panic("logic error")
	}
	mu := new(sync.RWMutex)
	cloned := &Segment{
		Type:      base.SORTED_SEG,
		MTBufMgr:  seg.MTBufMgr,
		SSTBufMgr: seg.SSTBufMgr,
		FsMgr:     seg.FsMgr,
		Meta:      meta,
		SLLNode:   *common.NewSLLNode(mu),
	}
	cloned.tree.RWMutex = mu
	cloned.tree.Blocks = make([]iface.IBlock, 0)
	cloned.tree.Helper = make(map[uint64]int)
	cloned.tree.BlockIds = make([]uint64, 0)

	indexHolder := td.GetIndexHolder().StrongRefSegment(seg.Meta.ID)
	if indexHolder == nil {
		panic("logic error")
	}

	id := seg.Meta.AsCommonID().AsSegmentID()
	segFile := seg.FsMgr.UpgradeFile(id)
	if segFile == nil {
		panic("logic error")
	}
	if indexHolder.Type == base.UNSORTED_SEG {
		indexHolder.Unref()
		indexHolder = td.GetIndexHolder().UpgradeSegment(seg.Meta.ID, base.SORTED_SEG)
		seg.IndexHolder.Init(segFile)
	}
	cloned.IndexHolder = indexHolder
	cloned.SegmentFile = segFile
	var prev iface.IBlock
	for _, blk := range seg.tree.Blocks {
		newBlkMeta, err := cloned.Meta.ReferenceBlock(blk.GetMeta().ID)
		if err != nil {
			panic(err)
		}
		cloned.Ref()
		cur, err := blk.CloneWithUpgrade(cloned, newBlkMeta)
		if err != nil {
			panic(err)
		}
		cloned.tree.Helper[newBlkMeta.ID] = len(cloned.tree.Blocks)
		cloned.tree.Blocks = append(cloned.tree.Blocks, cur)
		cloned.tree.BlockIds = append(cloned.tree.BlockIds, cur.GetMeta().ID)
		cloned.tree.BlockCnt++
		if prev != nil {
			cur.Ref()
			prev.SetNext(cur)
		}
		prev = cur
	}

	cloned.SegmentFile.Ref()
	cloned.Ref()
	cloned.OnZeroCB = cloned.close
	return cloned, nil
}

func (seg *Segment) UpgradeBlock(meta *md.Block) (iface.IBlock, error) {
	if seg.Type != base.UNSORTED_SEG {
		panic("logic error")
	}
	if seg.Meta.ID != meta.Segment.ID {
		panic("logic error")
	}
	idx, ok := seg.tree.Helper[meta.ID]
	if !ok {
		logutil2.Error("logic error")
		panic("logic error")
	}
	old := seg.tree.Blocks[idx]
	seg.Ref()
	upgradeBlk, err := old.CloneWithUpgrade(seg, meta)
	if err != nil {
		return nil, err
	}
	var oldNext iface.IBlock
	if idx != len(seg.tree.Blocks)-1 {
		oldNext = old.GetNext()
	}
	upgradeBlk.SetNext(oldNext)
	upgradeBlk.SetPrevVersion(old)

	seg.tree.Lock()
	defer seg.tree.Unlock()
	seg.tree.Blocks[idx] = upgradeBlk
	if idx > 0 {
		upgradeBlk.Ref()
		seg.tree.Blocks[idx-1].SetNext(upgradeBlk)
	}
	upgradeBlk.Ref()
	// old.SetNext(nil)
	old.Unref()
	return upgradeBlk, nil
}