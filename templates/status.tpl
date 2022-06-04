{{ define "title" }}vpsFree.cz Status{{ end }}

{{ define "content" }}
<div class="container">
	<div class="row">
		<div class="col">
			<h1>vpsFree.cz Status</h1>
			{{ .Notice }}
			<p>
				Rendered at: {{ .RenderedAt }}
			</p>
		</div>
	</div>

	<div class="row">
		<div class="col">
			<h2 class="text-{{ if .Status.VpsAdmin.IsOperational }}success{{ else }}danger{{ end }}">
				vpsAdmin {{ .Status.VpsAdmin.TotalUp }}/{{ .Status.VpsAdmin.TotalCount }}
			</h2>
			<table class="table">
				<thead>
					<tr>
						<th>Service</th>
						<th>Description</th>
						<th>Status</th>
					</tr>
				</thead>
				<tbody>
					<tr class="table-{{ if .Status.VpsAdmin.Webui.Status }}success{{ else }}danger{{ end }}">
						<td><a href="{{ .Status.VpsAdmin.Webui.Url }}">vpsadmin.vpsfree.cz</a></td>
						<td>Main web interface for vpsAdmin</td>
						<td>{{ if .Status.VpsAdmin.Webui.Status }}Operational{{ else }}Down{{ end }}</td>
					</tr>
					<tr class="table-{{ if .Status.VpsAdmin.Api.Status }}success{{ else }}danger{{ end }}">
						<td><a href="{{ .Status.VpsAdmin.Api.Url }}">api.vpsfree.cz</a></td>
						<td>HTTP API server</td>
						<td>{{ if .Status.VpsAdmin.Api.Status }}Operational{{ else }}Down{{ end }}</td>
					</tr>
					<tr class="table-{{ if .Status.VpsAdmin.Console.Status }}success{{ else }}danger{{ end }}">
						<td>Remote Console</td>
						<td>Interface for VPS remote console</td>
						<td>{{ if .Status.VpsAdmin.Console.Status }}Operational{{ else }}Down{{ end }}</td>
					</tr>
				</tbody>
			</table>
		</div>
	</div>

	{{ range $loc := .Status.LocationList }}
	<div class="row mt-4 mb-4">
		<div class="column">
			<h2 class="text-{{ if $loc.IsOperational }}success{{ else }}danger{{ end }}">
				{{ $loc.Label }} {{ $loc.TotalUp }}/{{ $loc.TotalCount }}
			</h2>

			<div class="accordion" id="accordion-location-{{ $loc.Id }}">
				<div class="accordion-item">
					<h2 class="accordion-header" id="heading-nodes-{{ $loc.Id }}">
						<button class="accordion-button text-{{ if $loc.NodesOperational }}success{{ else }}danger{{ end }}" type="button" data-bs-toggle="collapse" data-bs-target="#collapse-nodes-{{ $loc.Id }}" aria-expanded="true" aria-controls="collapse-nodes-{{ $loc.Id }}">
							Nodes {{ $loc.NodesUp }}/{{ $loc.NodesCount }}
						</button>
					</h2>
					<div id="collapse-nodes-{{ $loc.Id }}" class="accordion-collapse collapse show" aria-labelledby="heading-nodes-{{ $loc.Id }}">
						<div class="accordion-body">
							<div class="container">
								<div class="row">
									<div class="col">
										<table class="table">
											<thead>
												<tr>
													<th>Node</th>
													<th>vpsAdmin</th>
													<th>Ping</th>
												</tr>
											</thead>
											<tbody>
												{{ range $node := $loc.OddNodes }}
												<tr class="table-{{ if $node.IsOperational }}success{{ else }}danger{{ end }}">
													<td>{{ $node.Name }}</td>
													<td>
														{{ if $node.ApiStatus }}
															{{ if $node.ApiMaintenance }}Under maintenance{{ else }}Operational{{ end }}
														{{ else }}
															Down
														{{ end }}
													</td>
													<td>{{ if $node.Ping.IsUp }}Responding{{ else }}Down{{ end }}</td>
												</tr>
												{{ end }}
											</tbody>
										</table>
									</div>

									<div class="col">
										<table class="table">
											<thead>
												<tr>
													<th>Node</th>
													<th>vpsAdmin</th>
													<th>Ping</th>
												</tr>
											</thead>
											<tbody>
												{{ range $node := $loc.EvenNodes }}
												<tr class="table-{{ if $node.IsOperational }}success{{ else }}danger{{ end }}">
													<td>{{ $node.Name }}</td>
													<td>
														{{ if $node.ApiStatus }}
															{{ if $node.ApiMaintenance }}Under maintenance{{ else }}Operational{{ end }}
														{{ else }}
															Down
														{{ end }}
													</td>
													<td>{{ if $node.Ping.IsUp }}Responding{{ else }}Down{{ end }}</td>
												</tr>
												{{ end }}
											</tbody>
										</table>
									</div>
								</div>
							</div>
						</div>
					</div>
				</div> <!-- accordion item -->

				{{ if $loc.HasDnsResolvers }}
				<div class="accordion-item">
					<h2 class="accordion-header" id="heading-dns-{{ $loc.Id }}">
						<button class="accordion-button text-{{ if $loc.DnsResolversOperational }}success{{ else }}danger{{ end }}" type="button" data-bs-toggle="collapse" data-bs-target="#collapse-dns-{{ $loc.Id }}" aria-expanded="true" aria-controls="collapse-dns-{{ $loc.Id }}">
							DNS Resolvers {{ $loc.DnsResolversUp }}/{{ $loc.DnsResolversCount }}
						</button>
					</h2>
					<div id="collapse-dns-{{ $loc.Id }}" class="accordion-collapse collapse" aria-labelledby="heading-dns-{{ $loc.Id }}">
						<div class="accordion-body">
							<table class="table">
								<thead>
									<tr>
										<th>Name</th>
										<th>Ping</th>
										<th>Lookup</th>
									</tr>
								</thead>
								<tbody>
									{{ range $dns := $loc.DnsResolverList }}
									<tr class="table-{{ if $dns.IsOperational }}success{{ else }}danger{{ end }}">
										<td>{{ $dns.Name }}</td>
										<td>{{ if $dns.Ping.IsUp }}Responding{{ else }}Down{{ end }}</td>
										<td>{{ if $dns.ResolveStatus }}Operational{{ else }}Error{{ end }}</td>
									</tr>
									{{ end }}
								</tbody>
							</table>
						</div>
					</div>
				</div> <!-- accordion item -->
				{{ end }}

			</div>
		</div>
	</div>
	{{ end }}

	<footer class="py-3 my-4">
		<p class="text-end text-muted">vpsFree.cz</p>
	</footer>
</div>
{{ end }}
