{
  vpsadminPath,
  ...
}@args:
let
  seed = import (vpsadminPath + "/api/db/seeds/test-1-node.nix");
  nodeSeed = seed.node;
  location = seed.location;
  servicesAddress = "192.168.10.10";
  statusAddress = "192.168.10.30";
  socketNetwork = {
    type = "socket";
    mcast = {
      port = 22131;
    };
  };
  extraModules = args.extraModules or { };
  clusterArgs = args // {
    extraModules = extraModules // {
      services =
        { pkgs, ... }:
        {
          imports = if extraModules ? services then [ extraModules.services ] else [ ];

          environment.systemPackages = [
            pkgs.iptables
          ];

          vpsadmin.test.socketPeers = {
            "status.test" = statusAddress;
          };
        };
    };
  };
in
import ../make-test.nix (
  {
    pkgs,
    vpsfStatusModule,
    vpsfStatusPackage,
    ...
  }:
  {
    name = "status-page";

    description = ''
      Run vpsf-status against a local single-node vpsAdmin cluster and verify
      normal status reporting plus down/up recovery for node and vpsAdmin
      reachability.
    '';

    tags = [
      "ci"
      "status"
      "vpsadmin"
    ];

    machines =
      builtins.mapAttrs
        (
          _: machine:
          machine
          // {
            networks = map (
              network:
              if network.type == "socket" then socketNetwork else network
            ) machine.networks;
          }
        )
        (import (vpsadminPath + "/tests/machines/cluster/1-node.nix") clusterArgs)
      // {
        status = {
          spin = "nixos";
          memory = 1536;
          networks = [
            { type = "user"; }
            socketNetwork
          ];
          config =
            { ... }:
            {
              imports = [ vpsfStatusModule ];

              networking = {
                hostName = "vpsf-status";
                interfaces.eth1.useDHCP = false;
                interfaces.eth1.ipv4.addresses = [
                  {
                    address = statusAddress;
                    prefixLength = 24;
                  }
                ];
                hosts = {
                  "${servicesAddress}" = [
                    "api.vpsadmin.test"
                    "webui.vpsadmin.test"
                    "console.vpsadmin.test"
                  ];
                  "${nodeSeed.ipAddr}" = [
                    nodeSeed.name
                    nodeSeed.domain
                  ];
                };
              };

              environment.systemPackages = [
                pkgs.curl
              ];

              services.vpsf-status = {
                enable = true;
                package = vpsfStatusPackage;
                settings = {
                  check_interval = 2;
                  check_timeout = 2;
                  history_days = 7;
                  vpsadmin = {
                    api_url = "http://api.vpsadmin.test";
                    webui_url = "http://webui.vpsadmin.test";
                    console_url = "http://console.vpsadmin.test/console.js";
                  };
                  locations = [
                    {
                      id = location.id;
                      label = location.label;
                      nodes = [
                        {
                          id = nodeSeed.id;
                          name = nodeSeed.domain;
                          ip_address = nodeSeed.ipAddr;
                        }
                      ];
                      dns_resolvers = [ ];
                    }
                  ];
                  web_services = [ ];
                  nameservers = [ ];
                };
              };
            };
        };
      };

    testScript = ''
      require 'json'
      require 'shellwords'

      configure_examples do |config|
        config.default_order = :defined
      end

      STATUS_URL = 'http://127.0.0.1:8080'
      NODE_ID = ${toString nodeSeed.id}
      NODE_NAME = ${builtins.toJSON nodeSeed.domain}
      STATUS_IP = ${builtins.toJSON statusAddress}
      EXPECTED_POOL_STATE = 'unknown'
      EXPECTED_POOL_SCAN = 'unknown'

      def status_body(path)
        _, output = status.succeeds(
          "curl --silent --fail-with-body #{STATUS_URL}#{path}",
          timeout: 30
        )
        output
      end

      def status_json
        JSON.parse(status_body('/json'))
      end

      def status_metrics
        status_body('/metrics')
      end

      def find_node(document)
        document.fetch('locations').flat_map { |loc| loc.fetch('nodes') }.find do |node|
          node.fetch('name') == NODE_NAME
        end
      end

      def wait_for_status_json(name, timeout: 180)
        result = nil

        wait_until_block_succeeds(name: name, timeout: timeout) do
          result = status_json
          yield(result)
        end

        result
      end

      def wait_for_node_fields(fields, timeout: 180)
        wait_for_status_json("node fields #{fields.inspect}", timeout: timeout) do |document|
          node = find_node(document)
          node && fields.all? { |key, value| node.fetch(key.to_s) == value }
        end
      end

      def expect_metric(metrics, name, labels, value)
        line = metrics.lines.find do |candidate|
          candidate.start_with?("#{name}{") &&
            labels.all? { |key, val| candidate.include?(%Q(#{key}="#{val}")) }
        end

        expect(line).not_to be_nil
        expect(line).to match(/\s#{Regexp.escape(value.to_s)}(?:\.0)?(?:\s|$)/)
      end

      def expect_plain_metric(metrics, name, value)
        line = metrics.lines.find { |candidate| candidate.start_with?("#{name} ") }

        expect(line).not_to be_nil
        expect(line).to match(/\s#{Regexp.escape(value.to_s)}(?:\.0)?(?:\s|$)/)
      end

      def node_status_snapshot
        services.api_ruby_json(code: <<~RUBY)
          row = NodeCurrentStatus.find_by(node_id: #{NODE_ID})
          fresh_at = row && [row.updated_at, row.created_at].compact.max
          pool_checked_at = row && row.pool_checked_at
          puts JSON.dump(
            exists: !row.nil?,
            fresh: fresh_at && fresh_at > 2.minutes.ago,
            pool_fresh: pool_checked_at && pool_checked_at > 2.minutes.ago,
            updated_at: fresh_at&.utc&.iso8601,
            pool_checked_at: pool_checked_at&.utc&.iso8601
          )
        RUBY
      end

      def wait_for_fresh_node_status(timeout: 300)
        wait_until_block_succeeds(name: 'fresh vpsAdmin node status', timeout: timeout) do
          row = node_status_snapshot
          row.fetch('fresh') && row.fetch('pool_fresh')
        end
      end

      def wait_for_node_ready(timeout: 300)
        wait_until_block_succeeds(name: "node #{NODE_ID} ready in vpsAdmin", timeout: timeout) do
          row = services.api_ruby_json(code: <<~RUBY)
            node = Node.includes(:node_current_status).find(#{NODE_ID})
            puts JSON.dump(
              status: node.status,
              pool_status: node.pool_status,
              pool_state: node.pool_state_value
            )
          RUBY

          row.fetch('status') == true &&
            row.fetch('pool_status') == true
        end
      end

      def stale_node_status
        services.api_ruby_json(code: <<~RUBY)
          t = 10.minutes.ago
          row = NodeCurrentStatus.find_by!(node_id: #{NODE_ID})
          row.update_columns(
            created_at: t,
            updated_at: t,
            pool_checked_at: t
          )
          puts JSON.dump(ok: true)
        RUBY
      end

      def wait_for_running_nodectld
        node.wait_for_service('nodectld')

        wait_until_block_succeeds(name: "nodectld ready on #{NODE_NAME}", timeout: 300) do
          _, output = node.succeeds('sv check nodectld', timeout: 30)
          expect(output).to include('ok: run: nodectld')
          node.succeeds('test -S /run/nodectl/nodectld.sock', timeout: 30)
          node.succeeds('nodectl ping', timeout: 30)
          true
        end
      end

      def refresh_node_status(timeout: 180)
        node.wait_until_succeeds('nodectl refresh', timeout: timeout)
      end

      def block_node_ping
        node.succeeds(
          "iptables -I INPUT -s #{Shellwords.escape(STATUS_IP)} -p icmp --icmp-type echo-request -j DROP",
          timeout: 60
        )
      end

      def unblock_node_ping
        node.succeeds(
          "iptables -D INPUT -s #{Shellwords.escape(STATUS_IP)} -p icmp --icmp-type echo-request -j DROP || true",
          timeout: 60
        )
      end

      def block_services_http
        services.succeeds(
          "iptables -I INPUT -s #{Shellwords.escape(STATUS_IP)} -p tcp --dport 80 -j REJECT --reject-with tcp-reset",
          timeout: 60
        )
      end

      def unblock_services_http
        services.succeeds(
          "iptables -D INPUT -s #{Shellwords.escape(STATUS_IP)} -p tcp --dport 80 -j REJECT --reject-with tcp-reset || true",
          timeout: 60
        )
      end

      def create_outage(state:, summary:, outage_type: 'planned_outage')
        services.api_ruby_json(code: <<~RUBY)
          lang_en = Language.find_or_create_by!(code: 'en') do |lang|
            lang.label = 'English'
          end
          outage = Outage.create!(
            begins_at: Time.now.utc - 30.minutes,
            finished_at: #{state == 'resolved' ? 'Time.now.utc - 5.minutes' : 'nil'},
            duration: 30,
            state: #{state.inspect},
            outage_type: #{outage_type.inspect},
            impact_type: 'network'
          )
          OutageTranslation.create!(
            outage: outage,
            language: lang_en,
            summary: #{summary.inspect},
            description: 'Created by the vpsf-status integration test'
          )
          OutageEntity.create!(outage: outage, name: 'Node', row_id: #{NODE_ID})
          puts JSON.dump(id: outage.id, summary: #{summary.inspect})
        RUBY
      end

      before(:suite) do
        services.start
        services.wait_for_vpsadmin_api(timeout: 600)

        node.start
        wait_for_running_nodectld
        refresh_node_status
        wait_for_node_ready
        wait_for_fresh_node_status

        status.start
        status.wait_for_service('vpsf-status.service')
      end

      after(:suite) do
        unblock_node_ping if node.running?
        unblock_services_http if services.running?
        node.succeeds('sv start nodectld || true', timeout: 60) if node.running?
      end

      describe 'normal operation', order: :defined do
        it 'reports the expected test cluster state in JSON' do
          document = wait_for_status_json('operational JSON status', timeout: 240) do |json|
            node = find_node(json)

            json.dig('vpsadmin', 'api', 'status') == 'operational' &&
              json.dig('vpsadmin', 'webui', 'status') == 'operational' &&
              json.dig('vpsadmin', 'console', 'status') == 'operational' &&
              node &&
              node.fetch('id') == NODE_ID &&
              node.fetch('name') == NODE_NAME &&
              node.fetch('vpsadmin') == true &&
              node.fetch('ping') == 'responding' &&
              node.fetch('pool_state') == EXPECTED_POOL_STATE &&
              node.fetch('pool_scan') == EXPECTED_POOL_SCAN &&
              node.fetch('pool_status') == true
          end

          expect(document.dig('locations', 0, 'label')).to eq(${builtins.toJSON location.label})
        end

        it 'exports expected metrics' do
          metrics = nil

          wait_until_block_succeeds(name: 'healthy metrics', timeout: 120) do
            metrics = status_metrics

            expect_plain_metric(metrics, 'vpsfstatus_up', 1)
            expect_metric(metrics, 'vpsfstatus_vpsadmin_status', { 'service' => 'api' }, 0)
            expect_metric(metrics, 'vpsfstatus_vpsadmin_status', { 'service' => 'webui' }, 0)
            expect_metric(metrics, 'vpsfstatus_vpsadmin_status', { 'service' => 'console' }, 0)
            expect_metric(metrics, 'vpsfstatus_node_vpsadmin_status', { 'node_id' => NODE_ID, 'node_name' => NODE_NAME }, 0)
            expect_metric(metrics, 'vpsfstatus_node_ping_status', { 'node_id' => NODE_ID, 'node_name' => NODE_NAME }, 0)
            expect_metric(metrics, 'vpsfstatus_node_pool_status', { 'node_id' => NODE_ID, 'node_name' => NODE_NAME }, 0)
            expect_metric(metrics, 'vpsfstatus_node_pool_state', { 'node_id' => NODE_ID, 'node_name' => NODE_NAME }, 0)
            expect_metric(metrics, 'vpsfstatus_node_pool_scan', { 'node_id' => NODE_ID, 'node_name' => NODE_NAME }, 0)
          end
        end
      end

      describe 'node reachability changes', order: :defined do
        it 'detects node ping loss and recovery' do
          begin
            block_node_ping

            wait_for_node_fields({ ping: 'down', vpsadmin: true }, timeout: 90)
            wait_until_block_succeeds(name: 'node ping down metric', timeout: 30) do
              expect_metric(
                status_metrics,
                'vpsfstatus_node_ping_status',
                { 'node_id' => NODE_ID, 'node_name' => NODE_NAME },
                2
              )
            end
          ensure
            unblock_node_ping
          end

          wait_for_node_fields({ ping: 'responding', vpsadmin: true }, timeout: 120)
          wait_until_block_succeeds(name: 'node ping recovered metric', timeout: 30) do
            expect_metric(
              status_metrics,
              'vpsfstatus_node_ping_status',
              { 'node_id' => NODE_ID, 'node_name' => NODE_NAME },
              0
            )
          end
        end

        it 'detects nodectld reporting loss and recovery' do
          begin
            node.succeeds('sv stop nodectld', timeout: 60)
            node.wait_until_succeeds('sv status nodectld | grep -q "^down:"', timeout: 60)
            stale_node_status

            wait_for_node_fields(
              {
                ping: 'responding',
                vpsadmin: false,
                pool_status: false
              },
              timeout: 90
            )
          ensure
            node.succeeds('sv start nodectld || true', timeout: 60)
          end

          wait_for_running_nodectld
          refresh_node_status(timeout: 120)
          wait_for_fresh_node_status
          wait_for_node_fields(
            {
              ping: 'responding',
              vpsadmin: true,
              pool_status: true,
              pool_state: EXPECTED_POOL_STATE
            },
            timeout: 120
          )
        end
      end

      describe 'vpsAdmin reachability changes', order: :defined do
        it 'detects vpsAdmin HTTP loss and recovery' do
          begin
            block_services_http

            wait_for_status_json('vpsAdmin HTTP down status', timeout: 90) do |json|
              node = find_node(json)

              json.dig('vpsadmin', 'api', 'status') == 'down' &&
                json.dig('vpsadmin', 'webui', 'status') == 'down' &&
                json.dig('vpsadmin', 'console', 'status') == 'down' &&
                node &&
                node.fetch('ping') == 'responding' &&
                node.fetch('vpsadmin') == false
            end
          ensure
            unblock_services_http
          end

          wait_for_status_json('vpsAdmin HTTP recovered status', timeout: 120) do |json|
            node = find_node(json)

            json.dig('vpsadmin', 'api', 'status') == 'operational' &&
              json.dig('vpsadmin', 'webui', 'status') == 'operational' &&
              json.dig('vpsadmin', 'console', 'status') == 'operational' &&
              node &&
              node.fetch('ping') == 'responding' &&
              node.fetch('vpsadmin') == true
          end
        end
      end

      describe 'outage reports', order: :defined do
        it 'reports announced and recent outages' do
          active = create_outage(
            state: 'announced',
            summary: 'vpsf-status active planned outage'
          )
          recent = create_outage(
            state: 'resolved',
            summary: 'vpsf-status recent planned outage'
          )

          wait_for_status_json('outage reports', timeout: 90) do |json|
            announced = json.dig('outage_reports', 'announced')
            recent_reports = json.dig('outage_reports', 'recent')
            active_report = announced.find { |outage| outage.fetch('id') == active.fetch('id') }
            recent_report = recent_reports.find { |outage| outage.fetch('id') == recent.fetch('id') }

            active_report &&
              active_report.fetch('type') == 'planned_outage' &&
              active_report.fetch('state') == 'announced' &&
              active_report.fetch('impact') == 'network' &&
              active_report.fetch('en_summary') == active.fetch('summary') &&
              active_report.fetch('entities').any? { |entity| entity.fetch('name') == 'Node' && entity.fetch('id') == NODE_ID } &&
              recent_report &&
              recent_report.fetch('state') == 'resolved' &&
              recent_report.fetch('en_summary') == recent.fetch('summary')
          end
        end
      end
    '';
  }
) clusterArgs
