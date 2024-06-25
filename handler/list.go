package handler

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

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
    Stats []jFrogDownloadStats
}
type jFrogSearchReturn struct {
    Results []jFrogSearchResult `json:"results"`
}

var search_spec = `items.find(
    {
        "repo":{"$eq":"REPOSITORY"},
        "$and": [
            {"path":{"$match":"*/CLUSTER_MATCH/*"}},
            {"path":{"$match":"*/ARCH_MATCH/*"}},
            {"path":{"$match":"*/APP_MATCH/*"}},
            {"path":{"$match":"*/VERSION_MATCH/*"}},
            {"path":{"$nmatch":"*/sha256*"}}
        ]
    }
).include("name", "stat.downloads", "stat.downloaded", "repo", "path", "created", "sha256", "original_sha1", "actual_sha1", "size")`


func GetListHandler(config *util.Config) func(w http.ResponseWriter, r *http.Request) {
    return wrap(listHandler{config})
}

type listHandler struct {
    config *util.Config
}

func wildcard_default(s string) string {
    if s == "" {
        return "*"
    } else {
        return s
    }
}

func (h listHandler) Get(w http.ResponseWriter, r *http.Request) {
    searchHeaders := map[string]string{"Content-Type": "text/plain", "Authorization": fmt.Sprintf("Bearer %v", h.config.JFrog.Token)}
    searchTerm := search_spec
    searchTerm = strings.Replace(searchTerm, "REPOSITORY", h.config.JFrog.Repository, 1)
    searchTerm = strings.Replace(searchTerm, "CLUSTER_MATCH", wildcard_default(r.URL.Query().Get("cluster")), 1)
    searchTerm = strings.Replace(searchTerm, "ARCH_MATCH", wildcard_default(r.URL.Query().Get("arch")), 1)
    searchTerm = strings.Replace(searchTerm, "APP_MATCH", wildcard_default(r.URL.Query().Get("app")), 1)
    searchTerm = strings.Replace(searchTerm, "VERSION_MATCH", wildcard_default(r.URL.Query().Get("version")), 1)

    resp, err := util.DoRequest("POST", fmt.Sprintf("%v/artifactory/api/search/aql", h.config.JFrog.URL), searchHeaders, []byte(searchTerm))
    if err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
    }

    if err := util.CheckResponse(resp); err != nil {
        w.WriteHeader(resp.StatusCode)
        w.Write(resp.ResponseData)
        return
    }

    var searchReturn jFrogSearchReturn
    if err := json.Unmarshal(resp.ResponseData, &searchReturn); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
    }

    var ret jFrogSearchReturn
    reduced_sizes := make(map[string]int64)
    filename := "manifest.json"
    for _, res := range searchReturn.Results {
        reduced_sizes[res.Path] += res.Size
        if res.Name == filename {
            ret.Results = append(ret.Results, res)
        }
    }
    for idx := range ret.Results {
        ret.Results[idx].Size = reduced_sizes[ret.Results[idx].Path]
    }

    if respData, err := json.Marshal(ret); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
    } else {
        w.Write(respData)
    }
}
