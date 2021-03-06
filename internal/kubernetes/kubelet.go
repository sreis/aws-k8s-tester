package kubernetes

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

func sendKubeletPKI(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	rootCAPath string,
	kubeletConfig kubernetesconfig.Kubelet,
) (err error) {
	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	defer ss.Close()

	remotePath := fmt.Sprintf("/home/%s/kubelet.root.ca.crt", ec2Config.UserName)
	_, err = ss.Send(
		rootCAPath,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", rootCAPath, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}
	copyCmd := fmt.Sprintf("sudo mkdir -p %s && sudo cp %s %s", filepath.Dir(kubeletConfig.ClientCAFile), remotePath, kubeletConfig.ClientCAFile)
	_, err = ss.Run(
		copyCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", copyCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	return nil
}

func writeKubeletKubeConfigFile(
	privateKey []byte,
	publicKey []byte,
	rootCA []byte,
	internalServerURL string,
) (p string, err error) {
	cfg := clientcmdapi.NewConfig()
	cfg.APIVersion = "v1"
	cfg.Kind = "Config"
	cfg.Clusters["local"] = &clientcmdapi.Cluster{
		CertificateAuthorityData: rootCA,
		Server:                   internalServerURL,
	}
	cfg.Contexts["service-account-context"] = &clientcmdapi.Context{
		Cluster:  "local",
		AuthInfo: "kubelet",
	}
	cfg.CurrentContext = "service-account-context"
	cfg.AuthInfos["kubelet"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: publicKey,
		ClientKeyData:         privateKey,
	}
	var d []byte
	d, err = yaml.Marshal(&cfg)
	if err != nil {
		return "", err
	}
	p, err = fileutil.WriteTempFile(d)
	if err != nil {
		return "", fmt.Errorf("failed to write kubelet KUBECONFIG file (%v)", err)
	}
	return p, nil
}

func sendKubeletKubeConfigFile(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	filePathToSend string,
	kubeletConfig kubernetesconfig.Kubelet,
) (err error) {
	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	defer ss.Close()

	remotePath := fmt.Sprintf("/home/%s/kubelet.kubeconfig", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	copyCmd := fmt.Sprintf("sudo mkdir -p %s && sudo cp %s %s", filepath.Dir(kubeletConfig.Kubeconfig), remotePath, kubeletConfig.Kubeconfig)
	_, err = ss.Run(
		copyCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", copyCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	catCmd := fmt.Sprintf("sudo cat %s", kubeletConfig.Kubeconfig)
	var out []byte
	out, err = ss.Run(
		catCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil || len(out) == 0 {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", catCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	return nil
}

func writeKubeletEnvFile(target ec2config.Instance, kubeletConfig kubernetesconfig.Kubelet) (p string, err error) {
	kubeletConfig.HostnameOverride = target.PrivateDNSName
	var sc string
	sc, err = kubeletConfig.Sysconfig()
	if err != nil {
		return "", fmt.Errorf("failed to create kubelet sysconfig (%v)", err)
	}
	p, err = fileutil.WriteTempFile([]byte(sc))
	if err != nil {
		return "", fmt.Errorf("failed to write kubelet sysconfig file (%v)", err)
	}
	return p, nil
}

func sendKubeletEnvFile(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	filePathToSend string,
) (err error) {
	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	defer ss.Close()

	remotePath := fmt.Sprintf("/home/%s/kubelet.sysconfig", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	copyCmd := fmt.Sprintf("sudo mkdir -p /etc/sysconfig/ && sudo cp %s /etc/sysconfig/kubelet", remotePath)
	_, err = ss.Run(
		copyCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", copyCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	catCmd := "sudo cat /etc/sysconfig/kubelet"
	var out []byte
	out, err = ss.Run(
		catCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil || len(out) == 0 {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", catCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	return nil
}

func writeKubeletServiceFile(kubeletConfig kubernetesconfig.Kubelet) (p string, err error) {
	var sc string
	sc, err = kubeletConfig.Service()
	if err != nil {
		return "", fmt.Errorf("failed to create kubelet service file (%v)", err)
	}
	p, err = fileutil.WriteTempFile([]byte(sc))
	if err != nil {
		return "", fmt.Errorf("failed to write kubelet service file (%v)", err)
	}
	return p, nil
}

func sendKubeletServiceFile(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	filePathToSend string,
) (err error) {
	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	defer ss.Close()

	remotePath := fmt.Sprintf("/home/%s/kubelet.install.sh", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	remoteCmd := fmt.Sprintf("chmod +x %s && sudo bash %s", remotePath, remotePath)
	_, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	return nil
}
