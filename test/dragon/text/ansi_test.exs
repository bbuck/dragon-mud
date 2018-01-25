defmodule Dragon.Text.ANSITest do
  use ExUnit.Case
  doctest Dragon.Text.ANSI

  test "ansi_code returns expected values" do
    # Basic color codes
    assert Dragon.Text.ANSI.ansi_code("l", false) == "\e[30;22m"
    assert Dragon.Text.ANSI.ansi_code("-G", false) == "\e[42;1m"

    # xterm color codes
    assert Dragon.Text.ANSI.ansi_code("c102", false) == "\e[38;5;102m"
    assert Dragon.Text.ANSI.ansi_code("-c014", false) == "\e[48;5;14m"

    # xterm colors with fallback
    assert Dragon.Text.ANSI.ansi_code("c024", true) == "\e[36;22m"
    assert Dragon.Text.ANSI.ansi_code("-c238", true) == "\e[40;1m"

    # basic colors with fallback
    assert Dragon.Text.ANSI.ansi_code("r", true) == "\e[31;22m"
    assert Dragon.Text.ANSI.ansi_code("-C", true) == "\e[46;1m"
  end
end
