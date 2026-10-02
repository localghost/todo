package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
)

const usersUsage = "usage: todo users list | reset-password <name> | delete <name> [-yes]   (options: -db file)"

// runUsers runs the admin command "todo users …".
func runUsers(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usersUsage)
	}
	cmd, args := args[0], args[1:]
	fs := flag.NewFlagSet("users "+cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "todo.db", "path to the SQLite database file")
	yes := fs.Bool("yes", false, "delete without asking")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v; %s", err, usersUsage)
	}
	var name string
	if rest := fs.Args(); len(rest) > 0 {
		name = rest[0]
		if err := fs.Parse(rest[1:]); err != nil { // flags after the name
			return fmt.Errorf("%v; %s", err, usersUsage)
		}
		if fs.NArg() > 0 {
			return errors.New(usersUsage)
		}
	}
	needName := cmd == "reset-password" || cmd == "delete"
	switch {
	case cmd != "list" && !needName:
		return errors.New(usersUsage)
	case needName && name == "", cmd == "list" && name != "":
		return errors.New(usersUsage)
	}

	store, err := sqlite.OpenWith(*dbPath, sqlite.Options{RequireCurrent: true})
	if err != nil {
		return err
	}
	defer store.Close()
	ctx := context.Background()

	switch cmd {
	case "list":
		list, err := store.ListUsers(ctx, time.Now())
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "USERNAME\tCREATED\tITEMS\tSESSIONS")
		for _, u := range list {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", u.Username, u.CreatedAt.Local().Format("2006-01-02"), u.Items, u.Sessions)
		}
		return tw.Flush()
	case "reset-password":
		pw, err := auth.NewService(store).ResetPassword(ctx, name)
		if errors.Is(err, auth.ErrNoUser) {
			return fmt.Errorf("no user %q", name)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "New password for %s: %s\n", name, pw)
		return nil
	default: // delete
		u, err := store.UserByName(ctx, name)
		if errors.Is(err, auth.ErrNoUser) {
			return fmt.Errorf("no user %q", name)
		}
		if err != nil {
			return err
		}
		if !*yes {
			fmt.Fprintf(out, "Type the username to delete %s and all items: ", u.Username)
			line, _ := bufio.NewReader(in).ReadString('\n')
			if !strings.EqualFold(strings.TrimSpace(line), u.Username) {
				return errors.New("the typed name does not match; nothing was deleted")
			}
		}
		if err := store.DeleteUser(ctx, u.ID); err != nil {
			return err
		}
		fmt.Fprintf(out, "Deleted %s.\n", u.Username)
		return nil
	}
}
