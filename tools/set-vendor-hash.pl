use strict;
use warnings;

my ($hash, $path) = @ARGV;
die "Usage: perl tools/set-vendor-hash.pl <sha256-SRI|--fake> <file>\n"
  unless @ARGV == 2;
die "Expected a SHA-256 SRI hash or --fake.\n"
  unless $hash eq '--fake' || $hash =~ /\Asha256-[A-Za-z0-9+\/]{43}=\z/;

my $replacement = $hash eq '--fake' ? 'lib.fakeHash' : qq{"$hash"};
open my $input, '<', $path or die "Cannot read $path: $!\n";
my $source = do { local $/; <$input> };
close $input or die "Cannot close $path: $!\n";

# Refresh the default hash without changing the optional local-source override.
# Keep the supplied hash as data so its base64 slashes cannot become Perl syntax.
my $count = $source =~ s{
  ( ^[\t ]*vendorHash\s*=\s*
    (?: if\s+vpsadminGoClientSource\s*!=\s*null\s+then\s+
        "[^"\n]+"\s+else\s+
    )?
  )
  (?: lib\.fakeHash | "sha256-[A-Za-z0-9+/]{43}=" )\s*;
}{$1 . $replacement . ';'}gmex;
die "Expected one supported vendorHash assignment in $path.\n" unless $count == 1;

open my $output, '>', $path or die "Cannot write $path: $!\n";
print {$output} $source or die "Cannot write $path: $!\n";
close $output or die "Cannot close $path: $!\n";
