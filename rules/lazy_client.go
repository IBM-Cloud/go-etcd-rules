package rules

import (
	"errors"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

type LazyClient interface {
	LazySet(key, value string) error
	LazyGet(pattern string) (*string, error)
	List(pattern string, options *client.GetOptions) ([]*client.Node, error)
}

type lazyClient struct {
	api        client.KeysAPI
	attr       Attributes
	cancelFunc context.CancelFunc
	config     *client.Config
	client     client.Client
	timeout    time.Duration
}

func NewLazyClient(config *client.Config, attr Attributes, timeout time.Duration) LazyClient {
	return &lazyClient{
		config:  config,
		attr:    attr,
		timeout: timeout,
	}
}

func (lc *lazyClient) init() error {
	if lc.client == nil {
		c, err := client.New(*lc.config)
		if err != nil {
			return err
		}
		lc.client = c
	}
	if lc.api == nil {
		api := client.NewKeysAPI(lc.client)
		lc.api = api
	}
	return nil
}

func (lc *lazyClient) LazyGet(pattern string) (*string, error) {
	initErr := lc.init()
	if initErr != nil {
		return nil, initErr
	}
	key := lc.attr.Format(pattern)
	var originalValue *string
	defer lc.cancel()
	resp, err := lc.api.Get(lc.getContext(), key, nil)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "100") {
			return nil, err
		}
	} else {
		originalValue = &resp.Node.Value
	}
	return originalValue, nil
}

func (lc *lazyClient) LazySet(pattern, value string) error {
	initErr := lc.init()
	if initErr != nil {
		return initErr
	}
	key := lc.attr.Format(pattern)
	originalValue, getErr := lc.LazyGet(pattern)
	if getErr != nil {
		return getErr
	}
	if originalValue == nil || value != *originalValue {
		defer lc.cancel()
		_, err := lc.api.Set(lc.getContext(), key, value, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (lc *lazyClient) List(pattern string, options *client.GetOptions) ([]*client.Node, error) {
	initErr := lc.init()
	if initErr != nil {
		return []*client.Node{}, initErr
	}
	key := lc.attr.Format(pattern)
	defer lc.cancel()
	resp, err := lc.api.Get(lc.getContext(), key, options)
	if err != nil {
		return []*client.Node{}, err
	}
	if !resp.Node.Dir {
		return []*client.Node{}, errors.New("Path is not directory")
	}
	return resp.Node.Nodes, nil
}

func (lc *lazyClient) getContext() context.Context {
	ctx := context.Background()
	if lc.timeout > 0 {
		ctx, lc.cancelFunc = context.WithTimeout(ctx, lc.timeout)
	}
	return ctx
}

func (lc *lazyClient) cancel() {
	if lc.cancelFunc != nil {
		lc.cancelFunc()
		lc.cancelFunc = nil
	}
}
