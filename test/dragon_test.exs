defmodule DragonTest do
  use ExUnit.Case
  doctest Dragon

  test "greets the world" do
    assert Dragon.hello() == :world
  end
end
