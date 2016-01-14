package main

import (
	"github.com/joshlf/kudos/lib/dev"
	"github.com/joshlf/kudos/lib/kudos"
	"github.com/spf13/cobra"
)

var cmdAddAssignment = &cobra.Command{
	Use:   "add-assignment [code]",
	Short: "Add an assignment to the course database",
	// TODO(joshlf): long description
}

func init() {
	var forceFlag bool
	f := func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Usage()
			dev.Fail()
		}
		ctx := getContext()
		code := args[0]
		if err := kudos.ValidateCode(code); err != nil {
			ctx.Error.Printf("bad assignment code: %v\n", err)
			dev.Fail()
		}
		addCourseConfig(ctx)

		asgn, err := kudos.ParseAssignmentFileByCode(ctx, code)
		if err != nil {
			ctx.Error.Printf("could not read assignment config: %v\n", err)
			dev.Fail()
		}

		err = ctx.OpenDB()
		if err != nil {
			ctx.Error.Printf("could not open database: %v\n", err)
			dev.Fail()
		}
		defer kudos.CleanupDBAndLogOnError(ctx)

		changed := false
		ok := ctx.DB.AddAssignment(asgn)
		if ok {
			changed = true
		} else {
			if forceFlag {
				if len(ctx.DB.Grades[asgn.Code]) > 0 {
					ctx.Error.Printf("grades have been entered for assignment %v; in order to overwrite, first delete all grades for this assignment\n", asgn.Code)
					dev.Fail()
				}
				ctx.Warn.Printf("warning: overwriting assignment %v\n", asgn.Code)
				ctx.DB.DeleteAssignment(asgn.Code)
				ctx.DB.AddAssignment(asgn)
				changed = true
			} else {
				ctx.Warn.Printf("assignment %v already in database; use --force to overwrite\n", asgn.Code)
			}
		}

		if changed {
			err = ctx.CommitDB()
			if err != nil {
				ctx.Error.Printf("could not commit changes to database: %v\n", err)
				dev.Fail()
			}
		} else {
			err = ctx.CloseDB()
			if err != nil {
				ctx.Error.Printf("could not close database: %v\n", err)
				dev.Fail()
			}
		}
	}
	cmdAddAssignment.Run = f
	addAllGlobalFlagsTo(cmdAddAssignment.Flags())
	cmdAddAssignment.PersistentFlags().BoolVarP(&forceFlag, "force", "f", false, "overwrite previous version of assignment in database")
	cmdMain.AddCommand(cmdAddAssignment)
}