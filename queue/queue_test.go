package queue

import (
	"errors"
	"testing"
)

func TestUniqueQueue(t *testing.T) {

	t.Run("NewQueue", func(t *testing.T) {
		queue := NewQueue()

		if !queue.IsEmpty() {
			t.Error("expected an empty new queue")
		}

		if queue.Size() != 0 {
			t.Errorf("expected a queue with size 0, got %d", queue.Size())
		}
	})

	t.Run("Push", func(t *testing.T) {
		queue := NewQueue()

		tests := []struct {
			input string
			want  bool
			size  int
		}{
			{input: "item1", want: true, size: 1},
			{input: "item2", want: true, size: 2},
			{input: "item1", want: false, size: 2},
		}

		for _, test := range tests {
			got := queue.Push(test.input)
			if got != test.want {
				t.Errorf("input: %s, queue push got %t, want %t", test.input, got, test.want)
			}

			size := queue.Size()
			if size != test.size {
				t.Errorf("input: %s, queue size should be: %d, got: %d", test.input, test.size, size)
			}
		}
	})

	t.Run("PushForce", func(t *testing.T) {
		queue := NewQueue()

		tests := []struct {
			input string
			want  int
		}{
			{input: "item1", want: 1},
			{input: "item2", want: 2},
			{input: "item1", want: 3},
			{input: "item2", want: 4},
		}

		for _, test := range tests {
			queue.PushForce(test.input)

			size := queue.Size()
			if size != test.want {
				t.Errorf("input: %s, queue size should be: %d, got: %d", test.input, test.want, size)
			}
		}

	})

	t.Run("Pop", func(t *testing.T) {
		queue := NewQueue()

		tests := []struct {
			input    string
			want     string
			err      error
			useForce bool
		}{
			{input: "item1", want: "item1", err: nil, useForce: false},
			{input: "item2", want: "item2", err: nil, useForce: false},
			{input: "item1", want: "", err: ErrEmptyQueue, useForce: false},
			{input: "item2", want: "", err: ErrEmptyQueue, useForce: false},
			{input: "item1", want: "item1", err: nil, useForce: true},
			{input: "item2", want: "item2", err: nil, useForce: true},
			{input: "item2", want: "item2", err: nil, useForce: true},
		}

		for i, test := range tests {
			if test.useForce {
				queue.PushForce(test.input)
			} else {
				queue.Push(test.input)
			}

			got, err := queue.Pop()
			if got != test.want {
				t.Errorf("index# %d, Force push: %t. queue pop should be: %q, got: %q", i, test.useForce, test.want, got)
			}

			if !errors.Is(err, test.err) {
				t.Errorf("index# %d, Force push: %t. queue pop error should be: %q, got: %q", i, test.useForce, test.err, err)
			}
		}

	})

	t.Run("GetAndSetMapValue", func(t *testing.T) {
		queue := NewQueue()

		prepareQueue := []struct {
			input    string
			mapValue bool
		}{
			{input: "item1", mapValue: false},
			{input: "item2", mapValue: true},
		}
		for _, item := range prepareQueue {
			queue.Push(item.input)
			if item.mapValue {
				queue.SetMapValue(item.input, item.mapValue)
			}
		}

		tests := []struct {
			input string
			want  bool
			err   error
		}{
			{input: "ItemNeverPushed", want: false, err: ErrItemNotFound},
			{input: "item1", want: false, err: nil},
			{input: "item2", want: true, err: nil},
		}

		for _, test := range tests {
			got, err := queue.GetMapValue(test.input)

			if got != test.want {
				t.Errorf("input: %s, got queue getmap value: %t, wanted: %t", test.input, got, test.want)
			}

			if !errors.Is(err, test.err) {
				t.Errorf("input: %s, got queue error: %q, wanted: %q", test.input, err, test.err)
			}
		}
	})

	t.Run("View", func(t *testing.T) {
		queue := NewQueue()

		tests := []struct {
			input string
			want  string
			len   int
			err   error
		}{
			{input: "item1", want: "[item1]", len: 1, err: nil},
			{input: "item2", want: "[item1 item2]", len: 2, err: nil},
			{input: "", want: "", len: 99, err: ErrOutOfRange},
		}
		for _, item := range tests {
			if item.input != "" {
				queue.Push(item.input)
			}

			got, err := queue.View(item.len)

			if got != item.want {
				t.Errorf("input: %s, got: %q, wanted: %q", item.input, got, item.want)
			}

			if !errors.Is(err, item.err) {
				t.Errorf("input: %s, got error: %q, wanted: %q", item.input, err, item.err)
			}
		}

	})
}
