package main

import (
	"encoding/json"
	"fmt"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"
	"os"
	"os/user"
	"path"
	"strings"
	"time"
)

var (
	bucketName = []byte("secret")
	secretDB   *bbolt.DB
)

type Items struct {
	Items []Item `json:"items"`
}

type Item struct {
	Type         string `json:"type"`
	Title        string `json:"title"`
	Arg          string `json:"arg"`
	Autocomplete string `json:"autocomplete"`
}

func generateCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "generate google authenticator code",
		Long:  "generate google authenticator code",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			currentUser, err := user.Current()
			if err != nil {
				return err
			}
			filename := fmt.Sprintf("%s/%s", currentUser.HomeDir, ".google-authenticator/config.db")
			_, err = os.Stat(path.Dir(filename))
			if err != nil {
				if os.IsNotExist(err) {
					err = os.MkdirAll(path.Dir(filename), os.ModePerm)
					if err != nil {
						return err
					}
				}
			}
			secretDB, err = bbolt.Open(filename, 0666, nil)
			if err != nil {
				return err
			}
			return secretDB.Batch(func(tx *bbolt.Tx) error {
				_, err = tx.CreateBucketIfNotExists(bucketName)
				return err
			})
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return secretDB.Close()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := cmd.Flags().GetString("key")
			if err != nil {
				return err
			}
			return secretDB.View(func(tx *bbolt.Tx) error {
				data := tx.Bucket(bucketName).Get([]byte(key))
				if data == nil {
					return nil
				}
				code, err := totp.GenerateCode(string(data), time.Now())
				if err != nil {
					return err
				}
				fmt.Println(code)
				return nil
			})
		},
	}
	cmd.Flags().String("key", "", "secret key")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "query google authenticator secret",
		Long:  "query google authenticator secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			items := Items{Items: []Item{}}
			switch len(args) {
			case 0: // cmd query
				err := secretDB.View(func(tx *bbolt.Tx) error {
					return tx.Bucket(bucketName).ForEach(func(k, _ []byte) error {
						items.Items = append(items.Items, Item{
							Type:         "default",
							Title:        string(k),
							Arg:          fmt.Sprintf("--key %s", k),
							Autocomplete: fmt.Sprintf("--key %s", k),
						})
						return nil
					})
				})
				if err != nil {
					return err
				}
			case 1:
				switch strings.ToLower(args[0]) {
				case "add":
					items.Items = append(items.Items, Item{
						Type:         "default",
						Title:        "add",
						Arg:          "add",
						Autocomplete: "add",
					})
				case "del":
					err := secretDB.View(func(tx *bbolt.Tx) error {
						return tx.Bucket(bucketName).ForEach(func(k, _ []byte) error {
							items.Items = append(items.Items, Item{
								Type:         "default",
								Title:        fmt.Sprintf("del %s", string(k)),
								Arg:          fmt.Sprintf("del --key %s", string(k)),
								Autocomplete: fmt.Sprintf("del --key %s", string(k)),
							})
							return nil
						})
					})
					if err != nil {
						return err
					}
				default:
					err := secretDB.View(func(tx *bbolt.Tx) error {
						return tx.Bucket(bucketName).ForEach(func(k, _ []byte) error {
							if strings.Contains(string(k), args[0]) {
								items.Items = append(items.Items, Item{
									Type:         "default",
									Title:        string(k),
									Arg:          fmt.Sprintf("--key %s", k),
									Autocomplete: fmt.Sprintf("--key %s", k),
								})
							}
							return nil
						})
					})
					if err != nil {
						return err
					}
				}
			case 2:
				switch strings.ToLower(args[0]) {
				case "add": // cmd query add ...
					items.Items = append(items.Items, Item{
						Type:         "default",
						Title:        fmt.Sprintf("add %s", args[1]),
						Arg:          fmt.Sprintf("add --key %s", args[1]),
						Autocomplete: fmt.Sprintf("add --key %s", args[1]),
					})
				case "del":
					err := secretDB.View(func(tx *bbolt.Tx) error {
						return tx.Bucket(bucketName).ForEach(func(k, _ []byte) error {
							if strings.Contains(string(k), args[1]) {
								items.Items = append(items.Items, Item{
									Type:         "default",
									Title:        fmt.Sprintf("del %s", string(k)),
									Arg:          fmt.Sprintf("del --key %s", string(k)),
									Autocomplete: fmt.Sprintf("del --key %s", string(k)),
								})
							}
							return nil
						})
					})
					if err != nil {
						return err
					}
				}
			case 3:
				switch strings.ToLower(args[0]) {
				case "add": // cmd query add ...
					items.Items = append(items.Items, Item{
						Type:         "default",
						Title:        fmt.Sprintf("add %s %s", args[1], args[2]),
						Arg:          fmt.Sprintf("add --key %s --secret %s", args[1], args[2]),
						Autocomplete: fmt.Sprintf("add --key %s --secret %s", args[1], args[2]),
					})
				}
			}
			data, err := json.Marshal(items)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			return nil
		},
	}
	return cmd
}

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add google authenticator secret",
		Long:  "add google authenticator secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := cmd.Flags().GetString("key")
			if err != nil {
				return err
			}
			secret, err := cmd.Flags().GetString("secret")
			if err != nil {
				return err
			}
			_, err = totp.GenerateCode(secret, time.Now())
			if err != nil {
				return err
			}
			return secretDB.Batch(func(tx *bbolt.Tx) error {
				err = tx.Bucket(bucketName).Put([]byte(key), []byte(secret))
				if err != nil {
					return err
				}
				fmt.Printf("add [%s] secret success", key)
				return nil
			})
		},
	}
	cmd.Flags().String("key", "", "secret key")
	_ = cmd.MarkFlagRequired("key")
	cmd.Flags().String("secret", "", "secret data")
	_ = cmd.MarkFlagRequired("secret")
	return cmd
}

func delCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "del",
		Short: "del google authenticator secret",
		Long:  "del google authenticator secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := cmd.Flags().GetString("key")
			if err != nil {
				return err
			}
			return secretDB.Batch(func(tx *bbolt.Tx) error {
				err = tx.Bucket(bucketName).Delete([]byte(key))
				if err != nil {
					return err
				}
				fmt.Printf("del [%s] secret success", key)
				return nil
			})
		},
	}
	cmd.Flags().String("key", "", "secret key")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

func main() {
	rootCmd := generateCodeCmd()
	rootCmd.AddCommand(queryCmd(), addCmd(), delCmd())
	err := rootCmd.Execute()
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
}
