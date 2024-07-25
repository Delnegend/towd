// This package contains all the Discord Interaction handlers
//
// There should be 2 functions per handler, one for adding the handler &
// information to send to Discord (public), and one for handling the
// interaction (private).
//
// When need to add a temporary handler, use the `appState.AppCmdHandler`,
// remember to defer delete() the handler to clean up when done.
//
// Only return errors when it's the backend's fault, nil if user's fault.
package handler
