package alloc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/kouleen/common/client"
	"github.com/kouleen/idl/kitex_gen/workflow"
)

// SegmentAlloc 分布式号段分配器
type SegmentAlloc struct {
	bizTag string
	rec    *workflow.WorkflowSegmentResponse

	mu         sync.Mutex
	current    int64 // 当前待发放
	segmentEnd int64 // 当前号段结束(不含)

	startID int64
	endID   int64
}

// NewSegmentAlloc 创建分配器并预拉取第一段号段
func NewSegmentAlloc(ctx context.Context, bizTag string) (*SegmentAlloc, error) {
	status := int8(1)
	workflowSegmentList, err := client.GetWorkflowRpc(ctx).QueryWorkflowSegmentList(ctx, &workflow.WorkflowSegmentRequest{
		BizTag: bizTag,
		Status: &status,
	})
	if err != nil {
		return nil, err
	}
	if len(workflowSegmentList) == 0 {
		return nil, errors.New("no workflow segment found")
	}
	rec := workflowSegmentList[0]
	if err = ValidateUINRange(rec.GetStartId(), rec.GetEndId()); err != nil {
		return nil, fmt.Errorf("invalid uin range for biz tag %s: %w", bizTag, err)
	}
	s := &SegmentAlloc{
		rec:     rec,
		bizTag:  rec.GetBizTag(),
		startID: rec.GetStartId(),
		endID:   rec.GetEndId(),
	}
	if err = s.fetchSegment(ctx, rec); err != nil {
		return nil, fmt.Errorf("first fetch segment fail: %w", err)
	}
	return s, nil
}

// fetchSegment 事务+行锁原子抢占一段号段
func (s *SegmentAlloc) fetchSegment(ctx context.Context, rec *workflow.WorkflowSegmentResponse) error {
	s.mu.Lock()

	oldMax := rec.MaxId
	if oldMax >= s.endID {
		return errors.New("id pool exhausted")
	}
	newMax := oldMax + rec.Step
	if newMax > s.endID {
		newMax = s.endID
	}

	req := &workflow.WorkflowSegmentRequest{
		Id:         &rec.Id,
		BizTag:     rec.BizTag,
		MaxId:      &newMax,
		Step:       &rec.Step,
		StartId:    &rec.StartId,
		EndId:      &rec.EndId,
		Status:     rec.Status,
		Remark:     rec.Remark,
		IsDelete:   rec.IsDelete,
		CreatedBy:  &rec.CreatedBy,
		UpdatedBy:  &rec.UpdatedBy,
		CreateTime: &rec.CreateTime,
		UpdateTime: &rec.UpdateTime,
	}
	success, err := client.GetWorkflowRpc(ctx).FetchWorkflowSegment(ctx, req)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("fetch segment fail")
	}
	segmentResponse, err := client.GetWorkflowRpc(ctx).QueryWorkflowSegment(ctx, req)
	if err != nil {
		return err
	}
	s.rec = segmentResponse
	s.current = oldMax
	s.segmentEnd = newMax
	s.mu.Unlock()
	return nil
}

// Next 获取下一个全局唯一内部有序ID
func (s *SegmentAlloc) Next(ctx context.Context) (int64, error) {
	s.mu.Lock()
	if s.current >= s.segmentEnd {
		s.mu.Unlock()
		if err := s.fetchSegment(ctx, s.rec); err != nil {
			return 0, err
		}
		s.mu.Lock()
	}
	id := s.current
	s.current++
	s.mu.Unlock()

	if id < s.startID || id > s.endID {
		return 0, errors.New("id out of range")
	}
	return id, nil
}

func (s *SegmentAlloc) NextObfuscate(ctx context.Context) (int64, error) {
	next, err := s.Next(ctx)
	if err != nil {
		return 0, err
	}
	obfuscate, err := Obfuscate(next)
	if err != nil {
		return 0, err
	}
	return obfuscate, nil
}
