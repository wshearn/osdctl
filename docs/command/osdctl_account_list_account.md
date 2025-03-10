## osdctl account list account

List AWS Account CR

```
osdctl account list account [flags]
```

### Options

```
      --account-namespace string   The namespace to keep AWS accounts. The default value is aws-account-operator. (default "aws-account-operator")
  -c, --claim string               Filter account CRs by claimed or not. Supported values are true, false. Otherwise it lists all accounts
  -h, --help                       help for account
  -o, --output string              Output format. One of: json|yaml|jsonpath=...|jsonpath-file=... see jsonpath template [http://kubernetes.io/docs/user-guide/jsonpath].
  -r, --reuse string               Filter account CRs by reused or not. Supported values are true, false. Otherwise it lists all accounts
      --state string               Account cr state. The default value is all to display all the crs (default "all")
      --template string            Template string or path to template file to use when --output=jsonpath, --output=jsonpath-file.
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

* [osdctl account list](osdctl_account_list.md)	 - List resources

