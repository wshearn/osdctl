package access

import (
	"context"
	"fmt"
	"os"
	fpath "path/filepath"
	"strings"

	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/osdctl/pkg/k8s"
	osdctlutil "github.com/openshift/osdctl/pkg/utils"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func newCmdCleanup(streams genericclioptions.IOStreams, flags *genericclioptions.ConfigFlags) *cobra.Command {
	cleanupCmd := &cobra.Command{
		Use:               "cleanup <cluster identifier>",
		Short:             "Drop emergency access to a cluster",
		Long:              "Relinquish emergency access from the given cluster. If the cluster is PrivateLink, it deletes\nall jump pods in the cluster's namespace (because of this, you must be logged into the hive shard\nwhen dropping access for PrivateLink clusters). For non-PrivateLink clusters, the $KUBECONFIG\nenvironment variable is unset, if applicable.",
		Args:              cobra.ExactArgs(1),
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(cleanupCmdComplete(cmd, args))
			cmdutil.CheckErr(verifyPermissions(streams, flags))
			client := k8s.NewClient(flags)
			cleanupAccess := newCleanupAccessOptions(client, streams, flags)
			cmdutil.CheckErr(cleanupAccess.Run(cmd, args))
		},
	}
	return cleanupCmd
}

func cleanupCmdComplete(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmdutil.UsageErrorf(cmd, "Exactly one cluster identifier was expected")
	}
	return osdctlutil.IsValidClusterKey(args[0])
}

// cleanupAccessOptions contains the objects and information required to drop access to a cluster
type cleanupAccessOptions struct {
	*genericclioptions.ConfigFlags
	genericclioptions.IOStreams
	kclient.Client
}

// newCleanupAccessOptions creates a cleanupAccessOptions object
func newCleanupAccessOptions(client kclient.Client, streams genericclioptions.IOStreams, flags *genericclioptions.ConfigFlags) cleanupAccessOptions {
	c := cleanupAccessOptions{
		IOStreams:   streams,
		ConfigFlags: flags,
		Client:      client,
	}
	return c
}

// Println appends a newline then prints the given msg using the cleanupAccessOptions' IOStreams
func (c *cleanupAccessOptions) Println(msg string) {
	osdctlutil.StreamPrintln(c.IOStreams, msg)
}

// Println prints the given msg using the cleanupAccessOptions' IOStreams
func (c *cleanupAccessOptions) Print(msg string) {
	osdctlutil.StreamPrint(c.IOStreams, msg)
}

// Println appends a newline then prints the given error msg using the cleanupAccessOptions' IOStreams
func (c *cleanupAccessOptions) Errorln(msg string) {
	osdctlutil.StreamErrorln(c.IOStreams, msg)
}

// Readln reads a single line of user input using the cleanupAccessOptions' IOStreams. User input is returned with all
// proceeding and following whitespace trimmed
func (c *cleanupAccessOptions) Readln() (string, error) {
	in, err := osdctlutil.StreamRead(c.IOStreams, '\n')
	return strings.TrimSpace(in), err
}

// Run executes the 'cleanup' access subcommand
func (c *cleanupAccessOptions) Run(cmd *cobra.Command, args []string) error {
	clusteridentifier := args[0]

	conn := osdctlutil.CreateConnection()
	defer func() {
		cmdutil.CheckErr(conn.Close())
	}()

	cluster, err := osdctlutil.GetCluster(conn, clusteridentifier)
	if err != nil {
		return err
	}
	c.Println(fmt.Sprintf("Dropping access to cluster '%s'", cluster.Name()))
	if cluster.AWS().PrivateLink() {
		return c.dropPrivateLinkAccess(cluster)
	} else {
		return c.dropLocalAccess(cluster)
	}
}

// dropPrivateLinkAccess removes access to a PrivateLink cluster.
// This primarily consists of deleting any jump pods found to be running against the cluster in hive.
func (c *cleanupAccessOptions) dropPrivateLinkAccess(cluster *clustersmgmtv1.Cluster) error {
	c.Println("Cluster is PrivateLink - removing jump pods in the cluster's namespace.")
	ns, err := getClusterNamespace(c.Client, cluster.ID())
	if err != nil {
		c.Errorln("Failed to retrieve cluster namespace")
		return err
	}

	// Generate label selector to only target pods w/ matching jump pod label
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{jumpPodLabelKey: cluster.ID()}}
	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		c.Errorln("Failed to convert labelSelector to selector")
		return err
	}

	listOpts := kclient.ListOptions{Namespace: ns.Name, LabelSelector: selector}
	pods := corev1.PodList{}
	err = c.Client.List(context.TODO(), &pods, &listOpts)
	if err != nil {
		c.Errorln(fmt.Sprintf("Failed to list pods in cluster namespace '%s'", ns.Name))
		return err
	}

	numPods := len(pods.Items)
	if numPods == 0 {
		c.Println(fmt.Sprintf("No jump pods found running in namespace '%s'.", ns.Name))
		c.Println("Access has been dropped.")
		return nil
	}

	c.Println("")
	c.Println(fmt.Sprintf("This will delete %d pods in the namespace '%s'", numPods, ns.Name))
	for _, pod := range pods.Items {
		c.Println(fmt.Sprintf("- %s", pod.Name))
	}
	c.Println("")
	c.Print("Continue? [y/N] ")
	input, err := c.Readln()
	if err != nil {
		c.Errorln("Failed to read user input")
		return err
	}
	if isAffirmative(input) {
		pod := corev1.Pod{}
		err = c.Client.DeleteAllOf(context.TODO(), &pod, &kclient.DeleteAllOfOptions{ListOptions: listOpts})
		if err != nil {
			c.Errorln("Failed to delete pod(s)")
			return err
		}

		c.Println(fmt.Sprintf("Waiting for %d pod(s) to terminate", numPods))
		err = wait.PollImmediate(jumpPodPollInterval, jumpPodPollTimeout, func() (done bool, err error) {
			// For some reason, we have to recreate the podList after deleting the pods, otherwise the listOpts don't filter properly,
			// and we end up waiting for irrelevant pods. I've tried reproducing this bug in other places, but I haven't been able to
			// figure it out. If someone does, please fix it.
			pods := corev1.PodList{}
			err = c.Client.List(context.TODO(), &pods, &listOpts)
			if err != nil || len(pods.Items) != 0 {
				return false, err
			}
			return true, nil
		})
		if err != nil {
			c.Errorln("Error while waiting for pods to terminate")
			return err
		}
		c.Println("Access has been dropped.")
	} else {
		c.Println("Access has not been dropped.")
	}
	return nil
}

// dropLocalAccess removes access to a non-PrivateLink cluster.
// Basically it just unsets KUBECONFIG if it appears to be set to the given cluster, since we can't make assumptions
// around local files.
func (c *cleanupAccessOptions) dropLocalAccess(cluster *clustersmgmtv1.Cluster) error {
	c.Println("Unsetting $KUBECONFIG for cluster")
	kubeconfigPath, found := os.LookupEnv("KUBECONFIG")
	if !found {
		c.Errorln("'KUBECONFIG' unset. Access appears to have already been dropped.")
		return nil
	}

	kubeconfigFileName := fpath.Base(kubeconfigPath)
	if !strings.Contains(kubeconfigFileName, cluster.Name()) {
		c.Errorln(fmt.Sprintf("'KUBECONFIG' set to '%s', which does not seem to be the kubeconfig for '%s'. Access assumed to have already been dropped.", kubeconfigFileName, cluster.Name()))
		c.Errorln("(If you think this is a mistake, you can still manually drop access by running `unset KUBECONFIG` in the affected terminals)")
		return nil
	}

	c.Print(fmt.Sprintf("$KUBECONFIG set to '%s'. Unset it? [y/N]", kubeconfigPath))
	input, err := c.Readln()
	if err != nil {
		c.Errorln("Failed to read user input")
		return err
	}

	if isAffirmative(input) {
		c.Println("Unsetting $KUBECONFIG")
		err = os.Unsetenv("KUBECONFIG")
		if err != nil {
			c.Errorln("Failed to unset $KUBECONFIG")
			return err
		}
		c.Println("Successfully unset $KUBECONFIG.")
	}

	c.Println("Access has been dropped.")
	return nil
}
