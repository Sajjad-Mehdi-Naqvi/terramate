// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCloudConfig(t *testing.T) {
	type testcase struct {
		name      string
		layout    []string
		want      runExpected
		customEnv map[string]string
	}

	writeJSON := func(w http.ResponseWriter, str string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(str))
	}

	const fatalErr = `FTL ` + string(clitest.ErrCloud)

	for _, tc := range []testcase{
		{
			name: "empty cloud block == no organization set",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
						}
					}
				}`,
			},
			want: runExpected{
				Status: 1,
				StderrRegexes: []string{
					`Please set TM_CLOUD_ORGANIZATION environment variable`,
					fatalErr,
				},
			},
		},
		{
			name: "not a member of selected organization",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
							organization = "world"
						}
					}
				}`,
			},
			want: runExpected{
				Status: 1,
				StderrRegexes: []string{
					`You are not a member of organization "world"`,
					fatalErr,
				},
			},
		},
		{
			name: "member of organization",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
							organization = "mineiros-io"
						}
					}
				}`,
			},
			want: runExpected{
				Status: 0,
			},
		},
		{
			name: "cloud organization env var overrides value from config",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
							organization = "mineiros-io"
						}
					}
				}`,
			},
			customEnv: map[string]string{
				"TM_CLOUD_ORGANIZATION": "override",
			},
			want: runExpected{
				Status: 1,
				StderrRegexes: []string{
					`You are not a member of organization "override"`,
					fatalErr,
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			router := testserver.RouterWith(map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: false,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			})

			fakeserver := &http.Server{
				Handler: router,
				Addr:    "localhost:3001",
			}
			testserver.RouterAddCustoms(router, testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: http.HandlerFunc(
							func(w http.ResponseWriter, _ *http.Request) {
								writeJSON(w, `[
									{
										"org_name": "terramate-io",
										"org_display_name": "Terramate",
										"org_uuid": "c7d721ee-f455-4d3c-934b-b1d96bbaad17",
										"status": "active"
									},
									{
										"org_name": "mineiros-io",
										"org_display_name": "Mineiros",
										"org_uuid": "b2f153e8-ceb1-4f26-898e-eb7789869bee",
										"status": "active"
									}
								]`)
							},
						),
					},
				},
			})

			const fakeserverShutdownTimeout = 3 * time.Second
			errChan := make(chan error)
			go func() {
				errChan <- fakeserver.ListenAndServe()
			}()

			t.Cleanup(func() {
				err := fakeserver.Close()
				if err != nil {
					t.Logf("fakeserver HTTP Close error: %v", err)
				}
				select {
				case err := <-errChan:
					if err != nil && !errors.Is(err, http.ErrServerClosed) {
						t.Error(err)
					}
				case <-time.After(fakeserverShutdownTimeout):
					t.Error("time excedeed waiting for fakeserver shutdown")
				}
			})

			s := sandbox.New(t)
			layout := tc.layout
			if len(layout) == 0 {
				layout = []string{
					"s:stack:id=test",
				}
			}
			s.BuildTree(layout)
			s.Git().CommitAll("created stacks")
			env := removeEnv(os.Environ(), "CI")

			for k, v := range tc.customEnv {
				env = append(env, fmt.Sprintf("%v=%v", k, v))
			}

			tm := newCLI(t, s.RootDir(), env...)

			cmd := []string{
				"run",
				"--cloud-sync-deployment",
				"--", testHelperBin, "true",
			}
			assertRunResult(t, tm.run(cmd...), tc.want)
		})
	}
}
