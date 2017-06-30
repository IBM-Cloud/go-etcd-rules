package rules

import (
	"fmt"
	"log"
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

func TestCrawler(t *testing.T) {
	cfg, _, kapi := initEtcd()

	kapi.Set(context.Background(), "/root/child", "val1", nil)

	kp := testKeyProcessor{
		keys: []string{},
	}
	cr := etcdCrawler{
		baseCrawler: baseCrawler{
			kp:     &kp,
			logger: getTestLogger(),
			prefix: "/root",
		},
		kapi: kapi,
	}
	cr.singleRun(getTestLogger())
	assert.Equal(t, "/root/child", kp.keys[0])
	cr.prefix = "/notroot"
	cr.singleRun(getTestLogger())

	crawler, err := newCrawler(
		cfg,
		getTestLogger(),
		"/root",
		5,
		&kp,
		defaultWrapKeysAPI,
		0,
		[]string{"/a/b/c", "/d/e/f"},
	)
	assert.Equal(t, crawler.(*etcdCrawler).crawlGuides, [][]string{
		[]string{"", "a", "b", "c"},
		[]string{"", "d", "e", "f"},
	})
	assert.NoError(t, err)
	_, err = newCrawler(
		client.Config{},
		getTestLogger(),
		"/root",
		5,
		&kp,
		defaultWrapKeysAPI,
		10,
		[]string{},
	)
	assert.Error(t, err)
}

func TestV3Crawler(t *testing.T) {
	cfg, c := initV3Etcd()
	kapi := c
	kapi.Put(context.Background(), "/root/child", "val1")

	kp := testKeyProcessor{
		keys: []string{},
	}
	cr := v3EtcdCrawler{
		baseCrawler: baseCrawler{
			kp:     &kp,
			logger: getTestLogger(),
			prefix: "/root",
		},
		kv: c,
	}
	cr.singleRun(getTestLogger())
	assert.Equal(t, "/root/child", kp.keys[0])
	cr.prefix = "/notroot"
	cr.singleRun(getTestLogger())
	_, err := newV3Crawler(
		cfg,
		5,
		&kp,
		getTestLogger(),
		nil,
		0,
		"/root",
		defaultWrapKV,
		c,
	)
	assert.NoError(t, err)
}

func Test_expandCrawlGuides(t *testing.T) {
	testcases := []struct {
		crawlGuides         []string
		expandedCrawlGuides [][]string
	}{
		{
			crawlGuides:         []string{},
			expandedCrawlGuides: [][]string{},
		},
		{
			crawlGuides: []string{"a"},
			expandedCrawlGuides: [][]string{
				[]string{"a"},
			},
		},
		{
			crawlGuides: []string{"a/b/c"},
			expandedCrawlGuides: [][]string{
				[]string{"a", "b", "c"},
			},
		},
		{
			crawlGuides: []string{"a/b/c", "d/e/f"},
			expandedCrawlGuides: [][]string{
				[]string{"a", "b", "c"},
				[]string{"d", "e", "f"},
			},
		},
	}

	for i, testcase := range testcases {

		t.Run(fmt.Sprintf("TestCase_%v", i), func(t *testing.T) {

			actualExpandedCrawlGuides := expandCrawlGuides(testcase.crawlGuides)
			assert.Equal(t, testcase.expandedCrawlGuides, actualExpandedCrawlGuides)
		})
	}
}

func Test_matchesCrawlGuides(t *testing.T) {
	testcases := []struct {
		crawlGuides   [][]string
		testPath      string
		expectedMatch bool
	}{
		{
			crawlGuides:   [][]string{},
			testPath:      "",
			expectedMatch: true,
		},
		{
			crawlGuides:   [][]string{},
			testPath:      "/a/b",
			expectedMatch: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "a"},
			},
			testPath:      "/a",
			expectedMatch: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "b"},
			},
			testPath:      "/a",
			expectedMatch: false,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "a", "b"},
			},
			testPath:      "/a",
			expectedMatch: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "a", "b"},
			},
			testPath:      "/a/b/c",
			expectedMatch: false,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "x"},
				[]string{"", "a", "b"},
			},
			testPath:      "/a/b",
			expectedMatch: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "a", "x"},
				[]string{"", "a", "b"},
			},
			testPath:      "/a/b",
			expectedMatch: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "a", "b", ":c"},
			},
			testPath:      "/a/b/1234567890",
			expectedMatch: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "a", "b", ":c", "d"},
			},
			testPath:      "/a/b/1234567890/d",
			expectedMatch: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"", "a", "b", ":c", "d"},
			},
			testPath:      "/a/b/1234567890",
			expectedMatch: true,
		},
	}

	for _, testcase := range testcases {

		t.Run(testcase.testPath, func(t *testing.T) {
			crawler := etcdCrawler{
				crawlGuides: testcase.crawlGuides,
			}

			actualMatch := crawler.matchesCrawlGuides(testcase.testPath)

			assert.Equal(t, testcase.expectedMatch, actualMatch)
		})
	}
}

func Test_crawlPath(t *testing.T) {
	logger := zap.New(
		zap.NewJSONEncoder(zap.RFC3339Formatter("ts")),
		zap.AddCaller(),
		zap.DebugLevel,
	)

	testcases := []struct {
		crawlGuides     [][]string
		testPath        string
		expectedCrawled bool
	}{
		{
			crawlGuides:     [][]string{},
			testPath:        "",
			expectedCrawled: true,
		},
		{
			crawlGuides:     [][]string{},
			testPath:        "/a/b/c",
			expectedCrawled: true,
		},
		{
			crawlGuides: [][]string{
				[]string{"a", "b", "d"},
			},
			testPath:        "/a/b/c",
			expectedCrawled: false,
		},
	}

	for _, testcase := range testcases {

		t.Run(testcase.testPath, func(t *testing.T) {
			crawler := etcdCrawler{
				kapi:        &fakeKeysAPI{},
				crawlGuides: testcase.crawlGuides,
				baseCrawler: baseCrawler{
					kp: &testKeyProcessor{},
				},
			}

			crawler.crawlPath(testcase.testPath, logger)

			getCalls := crawler.kapi.(*fakeKeysAPI).GetCalls

			if testcase.expectedCrawled {
				assert.Equal(t, 1, len(getCalls))
				assert.Equal(t, testcase.testPath, getCalls[0])
			} else {
				assert.Equal(t, 0, len(getCalls))
			}
		})
	}
}

/* === FAKE KEYS API === */
type fakeKeysAPI struct {
	GetCalls []string
}

func (fakeKeysAPI *fakeKeysAPI) Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error) {
	fakeKeysAPI.GetCalls = append(fakeKeysAPI.GetCalls, key)
	return &client.Response{Node: &client.Node{Key: key, Value: ""}}, nil
}

func (fakeKeysAPI *fakeKeysAPI) Set(ctx context.Context, key, value string, opts *client.SetOptions) (*client.Response, error) {
	log.Fatal("Unimplemented fake Set")
	return nil, nil
}

func (fakeKeysAPI *fakeKeysAPI) Delete(ctx context.Context, key string, opts *client.DeleteOptions) (*client.Response, error) {
	log.Fatal("Unimplemented fake Delete")
	return nil, nil
}

func (fakeKeysAPI *fakeKeysAPI) Create(ctx context.Context, key, value string) (*client.Response, error) {
	log.Fatal("Unimplemented fake Create")
	return nil, nil
}

func (fakeKeysAPI *fakeKeysAPI) CreateInOrder(ctx context.Context, dir, value string, opts *client.CreateInOrderOptions) (*client.Response, error) {
	log.Fatal("Unimplemented fake CreateInOrder")
	return nil, nil
}

func (fakeKeysAPI *fakeKeysAPI) Update(ctx context.Context, key, value string) (*client.Response, error) {
	log.Fatal("Unimplemented fake Update")
	return nil, nil
}

func (fakeKeysAPI *fakeKeysAPI) Watcher(key string, opts *client.WatcherOptions) client.Watcher {
	log.Fatal("Unimplemented fake Watcher")
	return nil
}
