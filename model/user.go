package model

// A Role represents the (mutually exclusive) permissions of access.
type Role string

const (
	// ReadOnly role can read (GET requests).
	ReadOnly Role = "ReadOnly"
	// ReadWrite role can use read and write (all requests).
	ReadWrite Role = "ReadWrite"
)

// A User is an entity authorized to access this service.
type User struct {
	Name       string
	Permission []Role
}

// HasRequiredRoles checks if the user's permission has overlap with the required permissions.
func (user User) HasRequiredRoles(requiredRoles []Role) bool {
	// can do better than this double loop but it's not important at this time ...
	for _, requiredRole := range requiredRoles {
		for _, userRole := range user.Permission {
			if userRole == requiredRole {
				return true
			}
		}
	}
	return false
}

// Users represents a list of Users.
// TODO: actually this can just be a map of username:User.
type Users struct {
	Users []User
}

// FindUserByName finds a user by username, with the second return value indicating if a user a found.
func (users Users) FindUserByName(name string) (User, bool) {
	for _, user := range users.Users {
		if user.Name == name {
			return user, true
		}
	}
	return User{}, false
}
