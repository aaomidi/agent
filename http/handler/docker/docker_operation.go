package docker

import (
	"net/http"
	"strings"

	"bitbucket.org/portainer/agent"
	httperror "bitbucket.org/portainer/agent/http/error"
	"bitbucket.org/portainer/agent/http/proxy"
	"bitbucket.org/portainer/agent/http/response"
)

func (handler *Handler) dockerOperation(rw http.ResponseWriter, request *http.Request) *httperror.HandlerError {
	if handler.clusterService == nil {
		handler.dockerProxy.ServeHTTP(rw, request)
		return nil
	}

	managerOperationHeader := request.Header.Get(agent.HTTPManagerOperationHeaderName)

	if managerOperationHeader != "" {
		return handler.executeOperationOnManagerNode(rw, request)
	}

	return handler.dispatchOperation(rw, request)
}

func (handler *Handler) dispatchOperation(rw http.ResponseWriter, request *http.Request) *httperror.HandlerError {
	path := request.URL.Path

	switch {
	case path == "/containers/json":
		return handler.executeOperationOnCluster(rw, request)
	case path == "/images/json":
		return handler.executeOperationOnCluster(rw, request)
	case path == "/volumes" && request.Method == http.MethodGet:
		return handler.executeOperationOnCluster(rw, request)
	case path == "/networks" && request.Method == http.MethodGet:
		return handler.executeOperationOnCluster(rw, request)
	case strings.HasPrefix(path, "/services"):
		return handler.executeOperationOnManagerNode(rw, request)
	case strings.HasPrefix(path, "/tasks"):
		return handler.executeOperationOnManagerNode(rw, request)
	case strings.HasPrefix(path, "/secrets"):
		return handler.executeOperationOnManagerNode(rw, request)
	case strings.HasPrefix(path, "/configs"):
		return handler.executeOperationOnManagerNode(rw, request)
	case strings.HasPrefix(path, "/swarm"):
		return handler.executeOperationOnManagerNode(rw, request)
	case strings.HasPrefix(path, "/info"):
		return handler.executeOperationOnManagerNode(rw, request)
	case strings.HasPrefix(path, "/nodes"):
		return handler.executeOperationOnManagerNode(rw, request)
	default:
		return handler.executeOperationOnNode(rw, request)
	}
}

func (handler *Handler) executeOperationOnManagerNode(rw http.ResponseWriter, request *http.Request) *httperror.HandlerError {
	if handler.agentTags[agent.MemberTagKeyNodeRole] == agent.NodeRoleManager {
		handler.dockerProxy.ServeHTTP(rw, request)
	} else {
		targetMember := handler.clusterService.GetMemberByRole(agent.NodeRoleManager)
		if targetMember == nil {
			return &httperror.HandlerError{http.StatusInternalServerError, "The agent was unable to contact any other agent located on a manager node", agent.ErrManagerAgentNotFound}
		}
		proxy.AgentHTTPRequest(rw, request, targetMember)
	}
	return nil
}

func (handler *Handler) executeOperationOnNode(rw http.ResponseWriter, request *http.Request) *httperror.HandlerError {
	agentTargetHeader := request.Header.Get(agent.HTTPTargetHeaderName)

	if agentTargetHeader == handler.agentTags[agent.MemberTagKeyNodeName] || agentTargetHeader == "" {
		handler.dockerProxy.ServeHTTP(rw, request)
	} else {
		targetMember := handler.clusterService.GetMemberByNodeName(agentTargetHeader)
		if targetMember == nil {
			return &httperror.HandlerError{http.StatusInternalServerError, "The agent was unable to contact any other agent", agent.ErrAgentNotFound}
		}

		proxy.AgentHTTPRequest(rw, request, targetMember)
	}
	return nil
}

func (handler *Handler) executeOperationOnCluster(rw http.ResponseWriter, request *http.Request) *httperror.HandlerError {
	agentTargetHeader := request.Header.Get(agent.HTTPTargetHeaderName)

	if agentTargetHeader == handler.agentTags[agent.MemberTagKeyNodeName] {
		handler.dockerProxy.ServeHTTP(rw, request)
		return nil
	}

	clusterMembers := handler.clusterService.Members()

	data, err := handler.clusterProxy.ClusterOperation(request, clusterMembers)
	if err != nil {
		return &httperror.HandlerError{http.StatusInternalServerError, "Unable to execute cluster operation", err}
	}

	return response.JSON(rw, data)
}