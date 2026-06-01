# frozen_string_literal: true

require 'json'
require 'osvm'
require 'test-runner/hook'

class VpsadminServicesMachine < OsVm::NixosMachine
  def wait_for_vpsadmin_api(timeout: @default_timeout || 300)
    deadline = Time.now + timeout

    loop do
      raise OsVm::TimeoutError, 'Timed out waiting for vpsAdmin API' if Time.now >= deadline

      _, output = wait_until_succeeds(
        'curl --silent --fail-with-body http://api.vpsadmin.test/',
        timeout: [1, (deadline - Time.now).ceil].max
      )

      return true if output.include?('API description')

      sleep 1
    end
  end

  def api_ruby(code:, timeout: nil)
    script = <<~CMD
      set -euo pipefail
      api_dir="$(systemctl show -p WorkingDirectory --value vpsadmin-api)"
      api_root="$(dirname "$api_dir")"
      tmp_rb="$(mktemp /tmp/vpsf-status-it-XXXX.rb)"
      trap 'rm -f "$tmp_rb"' EXIT

      cat > "$tmp_rb" <<'RUBY'
      ENV['RACK_ENV'] ||= 'production'
      require 'json'
      Dir.chdir(ENV.fetch('API_DIR'))
      $LOAD_PATH.unshift(File.join(ENV.fetch('API_DIR'), 'lib'))
      require 'vpsadmin'
      plugin_root = File.expand_path('../plugins', ENV.fetch('API_DIR'))
      Dir[File.join(plugin_root, 'outage_reports', 'api', 'models', '*.rb')]
        .sort
        .each { |path| require path }
      #{code}
      RUBY

      API_DIR="$api_dir" "$api_root/ruby-env-wrapped/bin/ruby" "$tmp_rb"
    CMD

    timeout ? succeeds(script, timeout:) : succeeds(script)
  end

  def api_ruby_json(code:, timeout: nil)
    _, output = api_ruby(code:, timeout:)
    JSON.parse(output.to_s.lines.last)
  end
end

TestRunner::Hook.subscribe(:machine_class_for) do |machine_config|
  next unless machine_config.tags.include?('vpsadmin-services')

  VpsadminServicesMachine
end
