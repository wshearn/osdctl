## osdctl access
Relinquish emergency access from the given cluster. If the cluster is PrivateLink, it deletes all jump pods in the cluster's namespace (because of this, you must be logged into the hive shard when dropping access for PrivateLink clusters). For non-PrivateLink clusters, the $KUBECONFIG environment variable is unset, if applicable.


```
osdctl cluster break-glass cleanup <cluster identifier> [flags]
```

### Options

```
  -h, --help   help for cluster
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
      --as string                        Username to impersonate for the operation
      --cluster string                   The name of the kubeconfig cluster to use
      --context string                   The name of the kubeconfig context to use
      --insecure-skip-tls-verify         If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                Path to the kubeconfig file to use for CLI requests.
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --request-timeout string           The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                    The address and port of the Kubernetes API server
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [osdctl cluster break-glass](osdctl_cluster_break-glass.md)	 - Request or obtain emergency break-glass access for a cluster
