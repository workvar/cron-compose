package runtime

import (
	"github.com/croncompose/croncompose/agent/internal/osuser"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// handleListUsersRequest answers the control plane's "who can the web terminal
// switch to on this box" query. Sent on the direct path, same as terminal output and
// connector results: the caller is a blocked HTTP request, so a reply that arrives
// after a reconnect would be too late to matter.
func (r *Runtime) handleListUsersRequest(req *agentv1.ListUsersRequest) {
	if req == nil {
		return
	}

	res := &agentv1.ListUsersResult{RequestId: req.GetRequestId()}

	users, err := osuser.ListUsers()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Users = make([]*agentv1.SystemUser, 0, len(users))
		for _, u := range users {
			res.Users = append(res.Users, &agentv1.SystemUser{
				Username:  u.Username,
				Uid:       u.UID,
				Home:      u.Home,
				Shell:     u.Shell,
				Available: u.Available,
			})
		}
	}

	r.sendDirect(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_ListUsersResult{ListUsersResult: res},
	})
}
