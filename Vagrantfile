Vagrant.configure("2") do |config|
    config.vm.box = " generic/debian12"

    config.hostmanager.enabled = true
    config.hostmanager.manage_guest = true
    config.hostmanager.manage_host = false
    config.hostmanager.ignore_private_ip = true
    config.hostmanager.include_offline = true

    nodes = [
        { name: "control", role: "control", hostname: "control.trellis.local" },
        { name: "worker-1", role: "worker", hostname: "worker-1.trellis.local" },
        { name: "worker-2", role: "worker", hostname: "worker-2.trellis.local" }
    ]

    nodes.each do |definition|
        config.vm.define definition[:name] do |node|
            node.vm.hostname = definition[:hostname]

            node.vm.provider "hyperv" do |hyperv|
                hyperv.memory = 2048
                hyperv.cpus = 2
                hyperv.linked_clone = true
                hyperv.enable_virtualization_extensions = false
            end

            node.vm.provision "common", type: "shell", path: "demo/common.sh"
            if definition[:role] == "control"
                node.vm.provision "consul-server", type: "shell", path: "demo/consul-server.sh"
                node.vm.provision "trellis-server", type: "shell", path: "demo/trellis-server.sh"
            else
                node.vm.provision "containerd", type: "shell", path: "demo/containerd.sh"
                node.vm.provision "consul-client", type: "shell", path: "demo/consul-client.sh"
                node.vm.provision "trellis-agent", type: "shell", path: "demo/trellis-agent.sh"
            end
        end
    end
end