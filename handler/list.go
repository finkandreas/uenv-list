package handler

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    "sync"
    "time"

    "cscs.ch/uenv-list/util"
)

type jFrogDownloadStats struct {
    Downloaded string `json:"downloaded"`
    Downloads int64 `json:"downloads"`
}
type jFrogSearchResult struct {
    Repo string `json:"repo"`
    Path string `json:"path"`
    Name string `json:"name"`
    Size int64  `json:"size"`
    Created string `json:"created"`
    ActualSha1 string `json:"actual_sha1"`
    Sha256 string `json:"sha256"`
    SqfsSha256 string `json:"sqfs_sha256"`
    Stats []jFrogDownloadStats
}
type jFrogSearchReturn struct {
    Results []jFrogSearchResult `json:"results"`
}



var lastJFrogResults []jFrogSearchResult
var fetchMutex sync.Mutex
var lastFetchTimestamp int64

func GetListHandler(config *util.Config) func(w http.ResponseWriter, r *http.Request) {
    return wrap(listHandler{config})
}

type listHandler struct {
    config *util.Config
}

func (h listHandler) Get(w http.ResponseWriter, r *http.Request) {
    if lastJFrogResults == nil || time.Now().Unix() - lastFetchTimestamp > h.config.CacheTimeout {
        if returncode, err := h.fetchFromJFrog(); err != nil {
            w.WriteHeader(returncode)
            w.Write([]byte(err.Error()))
            return
        }
    }
    cluster_match := r.URL.Query().Get("cluster")
    arch_match := r.URL.Query().Get("arch")
    app_match := r.URL.Query().Get("app")
    version_match := r.URL.Query().Get("version")
    namespace_match := r.URL.Query().Get("namespace")

    var ret jFrogSearchReturn
    reduced_sizes := make(map[string]int64)
    sqfs_sha256 := make(map[string]string)
    filename := "manifest.json"
    for _, res := range lastJFrogResults {
        reduced_sizes[res.Path] += res.Size
        if res.Size > 100*1024*1024 {
            sqfs_sha256[res.Path] = res.Sha256
        }
        if res.Name == filename {
            // path == <namespace>/<cluster>/<arch>/<app>/<version>/TAG
            // by default if no namespace match is specified it will match "build" and "deploy" (to keep old behaviour)
            pathComponents := strings.Split(res.Path, "/")
            if len(pathComponents) >= 5 &&
              ( (namespace_match == "" && (pathComponents[0] == "build" || pathComponents[0] == "deploy")) ||
                (namespace_match == pathComponents[0]) ) &&
              (cluster_match == "" || pathComponents[1] == cluster_match) &&
              (arch_match == "" || pathComponents[2] == arch_match) &&
              (app_match == "" || pathComponents[3] == app_match) &&
              (version_match == "" || pathComponents[4] == version_match) {
                ret.Results = append(ret.Results, res)
            }
        }
    }
    for idx := range ret.Results {
        ret.Results[idx].Size = reduced_sizes[ret.Results[idx].Path]
        ret.Results[idx].SqfsSha256 = sqfs_sha256[ret.Results[idx].Path]
    }

    if respData, err := json.Marshal(ret); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
    } else {
        w.Write(respData)
    }
}

func (h listHandler) fetchFromJFrog() (int, error) {
    fetchMutex.Lock()
    defer fetchMutex.Unlock()
    if time.Now().Unix() - lastFetchTimestamp > h.config.CacheTimeout {
        start := time.Now().Unix()
        var search_spec = fmt.Sprintf(`items.find(
            {
                "repo":{"$eq":"%v"},
                "path":{"$nmatch":"*/sha256*"}
            }
        ).include("name", "stat.downloads", "stat.downloaded", "repo", "path", "created", "sha256", "original_sha1", "actual_sha1", "size")`, h.config.JFrog.Repository)

        searchHeaders := map[string]string{"Content-Type": "text/plain", "Authorization": fmt.Sprintf("Bearer %v", h.config.JFrog.Token)}

        resp, err := util.DoRequest("POST", fmt.Sprintf("%v/artifactory/api/search/aql", h.config.JFrog.URL), searchHeaders, []byte(search_spec))
        if err != nil {
            return 500, err
        }

        if err := util.CheckResponse(resp); err != nil {
            return resp.StatusCode, fmt.Errorf("%v", resp.ResponseData)
        }

        var searchReturn jFrogSearchReturn
        if err := json.Unmarshal(resp.ResponseData, &searchReturn); err != nil {
            return 500, err
        }

        lastFetchTimestamp = time.Now().Unix()
        lastJFrogResults = searchReturn.Results
        end := time.Now().Unix()
        log.Printf("Fetched results freshly from JFrog. The search took %v seconds\n", end-start)
    } else {
        // this happens if multiple goroutines hit the condition that the results are outdated, the first one locks the mutext
        // --> fetches results --> updates timestamp --> unlock mutex --> other goroutine locks mutex --> sees new fetch timestamp
        log.Println("The results have already been fetched by another goroutine. Not fetching again...")
    }
    return 200, nil
}
