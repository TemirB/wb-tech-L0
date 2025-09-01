package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/TemirB/wb-tech-L0/internal/domain"
)

func TestWarm(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := NewMockrepo(ctrl)
	cap := 3
	ids := []string{"1", "2", "3"}

	repo.EXPECT().RecentOrderIDs(gomock.Any(), cap).Return(ids, nil)
	for _, id := range ids {
		repo.EXPECT().GetByUID(gomock.Any(), id).Return(&domain.Order{OrderUID: id}, nil)
	}

	c, err := New(cap)
	if err != nil {
		t.Fatalf("unexpected error constructing cache: %v", err)
	}
	c.Warm(context.Background(), repo)

	for _, id := range ids {
		if _, ok := c.Get(id); !ok {
			t.Errorf("expected id %s to be cached after Warm", id)
		}
	}
}

func TestWarmIgnoresRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := NewMockrepo(ctrl)
	cap := 5

	repo.EXPECT().RecentOrderIDs(gomock.Any(), cap).Return(nil, errors.New("repo error"))
	repo.EXPECT().GetByUID(gomock.Any(), gomock.Any()).Times(0)

	c, err := New(cap)
	if err != nil {
		t.Fatalf("unexpected error constructing cache: %v", err)
	}

	c.Warm(context.Background(), repo) // должно отработать без паники
}

func TestWarmPartialErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := NewMockrepo(ctrl)
	cap := 4
	ids := []string{"ok1", "bad", "ok2"}

	repo.EXPECT().RecentOrderIDs(gomock.Any(), cap).Return(ids, nil)
	repo.EXPECT().GetByUID(gomock.Any(), "ok1").Return(&domain.Order{OrderUID: "ok1"}, nil)
	repo.EXPECT().GetByUID(gomock.Any(), "bad").Return(nil, errors.New("db read err"))
	repo.EXPECT().GetByUID(gomock.Any(), "ok2").Return(&domain.Order{OrderUID: "ok2"}, nil)

	c, err := New(cap)
	if err != nil {
		t.Fatalf("unexpected error constructing cache: %v", err)
	}
	c.Warm(context.Background(), repo)

	if _, ok := c.Get("ok1"); !ok {
		t.Errorf("ok1 must be cached")
	}
	if _, ok := c.Get("ok2"); !ok {
		t.Errorf("ok2 must be cached")
	}
	if _, ok := c.Get("bad"); ok {
		t.Errorf("bad must NOT be cached")
	}
}
