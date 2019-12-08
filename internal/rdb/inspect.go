package rdb

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Stats represents a state of queues at a certain time.
type Stats struct {
	Enqueued   int
	InProgress int
	Scheduled  int
	Retry      int
	Dead       int
	Timestamp  time.Time
}

// EnqueuedTask is a task in a queue and is ready to be processed.
// Note: This is read only and used for monitoring purpose.
type EnqueuedTask struct {
	ID      uuid.UUID
	Type    string
	Payload map[string]interface{}
}

// InProgressTask is a task that's currently being processed.
// Note: This is read only and used for monitoring purpose.
type InProgressTask struct {
	ID      uuid.UUID
	Type    string
	Payload map[string]interface{}
}

// ScheduledTask is a task that's scheduled to be processed in the future.
// Note: This is read only and used for monitoring purpose.
type ScheduledTask struct {
	ID        uuid.UUID
	Type      string
	Payload   map[string]interface{}
	ProcessAt time.Time
}

// RetryTask is a task that's in retry queue because worker failed to process the task.
// Note: This is read only and used for monitoring purpose.
type RetryTask struct {
	ID      uuid.UUID
	Type    string
	Payload map[string]interface{}
	// TODO(hibiken): add LastFailedAt time.Time
	ProcessAt time.Time
	ErrorMsg  string
	Retried   int
	Retry     int
}

// DeadTask is a task in that has exhausted all retries.
// Note: This is read only and used for monitoring purpose.
type DeadTask struct {
	ID           uuid.UUID
	Type         string
	Payload      map[string]interface{}
	LastFailedAt time.Time
	ErrorMsg     string
}

// CurrentStats returns a current state of the queues.
func (r *RDB) CurrentStats() (*Stats, error) {
	pipe := r.client.Pipeline()
	qlen := pipe.LLen(defaultQ)
	plen := pipe.LLen(inProgressQ)
	slen := pipe.ZCard(scheduledQ)
	rlen := pipe.ZCard(retryQ)
	dlen := pipe.ZCard(deadQ)
	_, err := pipe.Exec()
	if err != nil {
		return nil, err
	}
	return &Stats{
		Enqueued:   int(qlen.Val()),
		InProgress: int(plen.Val()),
		Scheduled:  int(slen.Val()),
		Retry:      int(rlen.Val()),
		Dead:       int(dlen.Val()),
		Timestamp:  time.Now(),
	}, nil
}

// ListEnqueued returns all enqueued tasks that are ready to be processed.
func (r *RDB) ListEnqueued() ([]*EnqueuedTask, error) {
	data, err := r.client.LRange(defaultQ, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	var tasks []*EnqueuedTask
	for _, s := range data {
		var msg TaskMessage
		err := json.Unmarshal([]byte(s), &msg)
		if err != nil {
			// continue // bad data, ignore and continue
			return nil, err
		}
		tasks = append(tasks, &EnqueuedTask{
			ID:      msg.ID,
			Type:    msg.Type,
			Payload: msg.Payload,
		})
	}
	return tasks, nil
}

// ListInProgress returns all tasks that are currently being processed.
func (r *RDB) ListInProgress() ([]*InProgressTask, error) {
	data, err := r.client.LRange(inProgressQ, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	var tasks []*InProgressTask
	for _, s := range data {
		var msg TaskMessage
		err := json.Unmarshal([]byte(s), &msg)
		if err != nil {
			continue // bad data, ignore and continue
		}
		tasks = append(tasks, &InProgressTask{
			ID:      msg.ID,
			Type:    msg.Type,
			Payload: msg.Payload,
		})
	}
	return tasks, nil
}

// ListScheduled returns all tasks that are scheduled to be processed
// in the future.
func (r *RDB) ListScheduled() ([]*ScheduledTask, error) {
	data, err := r.client.ZRangeWithScores(scheduledQ, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	var tasks []*ScheduledTask
	for _, z := range data {
		s, ok := z.Member.(string)
		if !ok {
			continue // bad data, ignore and continue
		}
		var msg TaskMessage
		err := json.Unmarshal([]byte(s), &msg)
		if err != nil {
			continue // bad data, ignore and continue
		}
		processAt := time.Unix(int64(z.Score), 0)
		tasks = append(tasks, &ScheduledTask{
			ID:        msg.ID,
			Type:      msg.Type,
			Payload:   msg.Payload,
			ProcessAt: processAt,
		})
	}
	return tasks, nil
}

// ListRetry returns all tasks that have failed before and willl be retried
// in the future.
func (r *RDB) ListRetry() ([]*RetryTask, error) {
	data, err := r.client.ZRangeWithScores(retryQ, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	var tasks []*RetryTask
	for _, z := range data {
		s, ok := z.Member.(string)
		if !ok {
			continue // bad data, ignore and continue
		}
		var msg TaskMessage
		err := json.Unmarshal([]byte(s), &msg)
		if err != nil {
			continue // bad data, ignore and continue
		}
		processAt := time.Unix(int64(z.Score), 0)
		tasks = append(tasks, &RetryTask{
			ID:        msg.ID,
			Type:      msg.Type,
			Payload:   msg.Payload,
			ErrorMsg:  msg.ErrorMsg,
			Retry:     msg.Retry,
			Retried:   msg.Retried,
			ProcessAt: processAt,
		})
	}
	return tasks, nil
}

// ListDead returns all tasks that have exhausted its retry limit.
func (r *RDB) ListDead() ([]*DeadTask, error) {
	data, err := r.client.ZRangeWithScores(deadQ, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	var tasks []*DeadTask
	for _, z := range data {
		s, ok := z.Member.(string)
		if !ok {
			continue // bad data, ignore and continue
		}
		var msg TaskMessage
		err := json.Unmarshal([]byte(s), &msg)
		if err != nil {
			continue // bad data, ignore and continue
		}
		lastFailedAt := time.Unix(int64(z.Score), 0)
		tasks = append(tasks, &DeadTask{
			ID:           msg.ID,
			Type:         msg.Type,
			Payload:      msg.Payload,
			ErrorMsg:     msg.ErrorMsg,
			LastFailedAt: lastFailedAt,
		})
	}
	return tasks, nil
}