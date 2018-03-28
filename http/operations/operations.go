package operations

import (
	"net/http"
	"sync"

	"bitbucket.org/portainer/agent"
)

func NodeWSOperation(w http.ResponseWriter, request *http.Request, member *agent.ClusterMember) {
	u := request.URL
	// TODO: member.AgentPort is in the address format here (:9001), could be a real IP address.
	// Fix that.
	u.Host = member.IPAddress + member.Port

	// TODO: will this work with TLS comms between agents?
	u.Scheme = "ws"
	// if request.TLS != nil {
	// 	url.Scheme = "https"
	// }

	websocketReverseProxy(w, request, u)
}

func NodeOperation2(w http.ResponseWriter, request *http.Request, member *agent.ClusterMember) {
	url := request.URL
	// TODO: member.AgentPort is in the address format here (:9001), could be a real IP address.
	// Fix that.
	url.Host = member.IPAddress + member.Port

	// TODO: figure out if this is the best way to determine scheme
	url.Scheme = "http"
	if request.TLS != nil {
		url.Scheme = "https"
	}

	reverseProxy(w, request, url)
}

// TODO: replaced by NodeOperation2, make sure everything is OK and check if that's actually
// a better way to do this.
func NodeOperation(request *http.Request, member *agent.ClusterMember) (*http.Response, error) {
	return executeRequestOnClusterMember(request, member)
}

func ClusterOperation(request *http.Request, clusterMembers []agent.ClusterMember) ([]interface{}, error) {

	memberCount := len(clusterMembers)

	// we create a slice with a capacity of memberCount but 0 size
	// so we'll avoid extra unneeded allocations
	data := make([]interface{}, 0, memberCount)

	// we create a buffered channel so writing to it won't block while we wait for the waitgroup to finish
	ch := make(chan parallelRequestResult, memberCount)

	// we create a waitgroup - basically block until N tasks say they are done
	wg := sync.WaitGroup{}

	for i := range clusterMembers {
		//we add 1 to the wait group - each worker will decrease it back
		wg.Add(1)

		member := &clusterMembers[i]

		go executeParallelRequest(request, member, ch, &wg)
	}

	// now we wait for everyone to finish - again, not a must.
	// you can just receive from the channel N times, and use a timeout or something for safety

	// TODO: a timeout should be used to here (or when executing HTTP requests)
	// to avoid blocking if one of the agent is not responding
	wg.Wait()

	// we need to close the channel or the following loop will get stuck
	close(ch)

	// we iterate over the closed channel and receive all data from it

	// TODO: find a way to manage any error that would be raised in a parallel request
	// It's available in the result.err field
	for result := range ch {
		for _, JSONObject := range result.data {

			metadata := agent.AgentMetadata{}
			metadata.Agent.NodeName = result.nodeName

			object := JSONObject.(map[string]interface{})
			object[agent.ResponseMetadataKey] = metadata

			data = append(data, object)
		}
	}

	return data, nil
}
