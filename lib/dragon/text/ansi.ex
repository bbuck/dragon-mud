defmodule Dragon.Text.ANSI do
  @moduledoc ~S"""
  The ANSI module is devoted to converting dragon codes into ANSI codes
  that can be used to colorize and modify text. It contains various functions
  to convert dragon codes to ANSI codes, apply to entire strings as well as
  escaping codes such that they can be rendered in strings without replacement
  happening.

  ## Dragon Codes
  
  To simply ANSI codes for users this module uses "dragon codes" to determine
  the appropriate ANSI code. For example, red text is `[r]` using dragon 
  codes and `\e[31;22m` with ANSI (the `\e[31m` is sufficient but the extra
  `;22` is an added nicety that enabled bold and non-bold colors to appear
  next to each other, another thing that dragon codes encapsulate). Dragon codes
  must be wrapped in square brackets within a string and can be easily
  escaped by double the square brackets (as a side effect of simple regex patterns
  you only need to double _one_ set of square brackets, but both is the ideal
  and intentional pattern).

  The set of dragon codes for normal ANSI color codes are as follows:

   - `l`/`L` for black and bold black respectively
   - `r`/`R` for red and bold red respectively
   - `g`/`G` for green and bold green respectively
   - `y`/`Y` for yellow and bold yellow respectively
   - `b`/`B` for blue and bold blue respectively
   - `m`/`M` for magenta and bold magenta respectively
   - `c`/`C` for cyan and bold cyan respectively
   - `w`/`W` for white and bold white respectively

  Dragon codes also support xterm colors by using a `c` and then three digits
  representing a numeric value from 0 to 255. For example, `c024` maps to
  `\e[38;5;24m`. Just another example in how the dragon codes are simpler
  to use.

      iex> Dragon.Text.ANSI.ansi_code("c")
      "\e[36;22m"

      iex> Dragon.Text.ANSI.parse("[r]red text[x]")
      "\e[31;22mred text\e[0m"

      iex> Dragon.Text.ANSI.parse("other [c123]color[x]")
      "other \e[38;5;123mcolor\e[0m"

   You can escape codes by adding another set of square brackets around them
   as mentioned earlier.

      iex> Dragon.Text.ANSI.parse("escaped [[x]] code")
      "escaped [x] code"

      iex> Dragon.Text.ANSI.parse("not [[x] escaped")
      "not [\e[0m escaped"

      iex> Dragon.Text.ANSI.parse("not [x]] escaped")
      "not \e[0m] escaped"

  Note that only the fully escaped is actually escaped in the output. 

  If you have a string with dragon codes and you want to preserve them (escape) so
  they wont get parsed, you can use `escape/1`.

      iex> Dragon.Text.ANSI.escape("[r]red text[x]")
      "[[r]]red text[[x]]"

      iex> Dragon.Text.ANSI.parse(Dragon.Text.ANSI.escape("[r]red text[x]"))
      "[r]red text[x]"

  """

  @typedoc """
  A valid color code supported by the Dragon color engine, such as the
  single character codes like 'r', or "B" or xterm codes like "c102" 
  or "c182". While just an alias for a String this type should be preferred
  if the string you expect is a valid dragon supported code character.
  """
  @type dragon_code() :: String.t()

  @typedoc """
  Represents an ANSI code, which is a text based code starting with an
  escape character ('\e') and containing numbers, semicolons and 
  concluding with an 'm' While just a simple alias for String.t() this
  type should be prefered to be explicit that you except an ansi_code.
  """
  @type ansi() :: Strint.t()

  @dragon_code_rx ~r/\[{1,2}(?:(?<simple>-?[lrgybmcwLRGYBMCWxu~])|(?<xterm>c\d{3}))\]{1,2}/

  @base_colors %{
    "l" => "0;22", "L" => "0;1",
    "r" => "1;22", "R" => "1;1",
    "g" => "2;22", "G" => "2;1",
    "y" => "3;22", "Y" => "3;1",
    "b" => "4;22", "B" => "4;1",
    "m" => "5;22", "M" => "5;1",
    "c" => "6;22", "C" => "6;1",
    "w" => "7;22", "W" => "7;1"
  }

  @ansi_code_map %{
    "x" => "\e[0m",
    "u" => "\e[4m",
    "~" => "\e[7m"
  }

  @ansi_code_map Enum.map(@base_colors, fn {code, color} ->
    [{code, "\e[3" <> color <> "m"},
     {"-" <> code, "\e[4" <> color <> "m"}]
  end) |> List.flatten() |> Enum.into(@ansi_code_map)

  @ansi_code_map (
    Enum.map(0..255, fn c ->
      key = "c" <> String.pad_leading(to_string(c), 3, "0")
      num_str = to_string(c)
      [
        {key, "\e[38;5;" <> num_str <> "m"},
        {"-" <> key, "\e[48;5;" <> num_str <> "m"}
      ]
    end) |> List.flatten() |> Enum.into(@ansi_code_map)
  )

  @fallback_map [
    {"l", [0]},
    {"r", [1, 52, 88, {124, 125}]},
    {"g", [2, 22, {28, 29}, {34, 35}, {40, 42}, 71]},
    {"y", [3, 58, {64, 65}, {94, 95}, {100, 101}, {106, 107}, {130, 131},
           {136, 137}, {142, 143}, {148, 150}, 166, {172, 173}, {178, 179},
           {186, 187}, {220, 222}]},
    {"b", [4, {17, 19}, 60]},
    {"m", [5, {53, 56}, {89, 93}, {96, 97}, {126, 129}, {132, 135}, {139, 141},
           {162, 163}, {167, 170}, {174, 177}]},
    {"c", [6, {23, 24}, {30, 31}, {36, 39}, {43, 45}, {66, 67}, {72, 75}, 
           {115, 116}]},
    {"w", [7, 59, {108, 109}, 138, {144, 147}, {151, 152}, {180, 183}, {188, 189},
           {192, 195}, {218, 219}, {223, 225}, {241, 250}]},
    {"L", [8, 16, {232, 240}]},
    {"R", [9, {160, 161}, {196, 199}, {202, 203}, {208, 211}]},
    {"G", [10, {46, 48}, 70, {76, 78}, {82, 84}, {112, 114}, {118, 121}, {154, 157},
           {190, 191}]},
    {"Y", [11, {184, 185}, {214, 217}, {226, 229}]},
    {"B", [12, {20, 21}, {25, 27}, {32, 33}, 57, {61, 63}, {68, 69}, {98, 99},
           {103, 105}, {110, 111}]},
    {"M", [13, {164, 165}, 171, {200, 201}, {204, 207}, {212, 213}]},
    {"C", [14, {49, 51}, {79, 81}, {85, 87}, 117, {122, 123}, 153, {158, 159}]},
    {"W", [15, 102, {230, 231}, {251, 255}]}
  ]

  @fallback_map (
    Enum.map(@fallback_map, fn {fallback, codes} ->
      Enum.map(codes, fn
        x when is_integer(x) ->
          key = "c" <> String.pad_leading(to_string(x), 3, "0")
          [
            {key, fallback},
            {"-" <> key, "-" <> fallback}
          ]

        {from, to} ->
          Enum.map(from..to, fn x ->
            key = "c" <> String.pad_leading(to_string(x), 3, "0")
            [
              {key, fallback},
              {"-" <> key, "-" <> fallback}
            ]
          end)
      end)
    end) |> List.flatten() |> Enum.into(%{})
  )
  
  @doc """
  List of dragon codes (those relating to xterm codes) mapped to basic
  dragon codes that should be used in place of the xterm codes when 
  fallback codes are requested.
  """
  @spec fallback_map() :: %{required(dragon_code()) => dragon_code()}
  def fallback_map, do: @fallback_map

  @doc "Maps dragon codes to ANSI codes."
  @spec fallback_map() :: %{required(dragon_code()) => ansi()}
  def ansi_code_map, do: @ansi_code_map

  @spec dragon_code_rx() :: Regex.t()
  def dragon_code_rx, do: @dragon_code_rx

  @doc """
  Given a dragon code value this will return the non-fallback ANSI 
  equivalent or nil if none is found in the map. This method simply
  forwards to `ansi_code/2` with `false` specified as the fallback.
  It's usually a better idea to use `ansi_code/2` directly.
  """
  @spec ansi_code(dragon_code()) :: ansi() | nil
  def ansi_code(code), do: ansi_code(code, false)

  @doc """
  Given a dragon_code this will return the equivalent ANSI code 
  associated to it, if the fallback is `false`.
  """
  @spec ansi_code(dragon_code(), boolean()) :: ansi() | nil
  def ansi_code(code, false), do: Map.get(ansi_code_map(), code)

  @doc """
  Given a dragon code this function attempts to target the fallback
  for the given dragon code and return the associated ANSI code for
  the fallback. If no fallback is discovered this method attempts a
  non-fallback lookup of the dragon_code.
  """ 
  @spec ansi_code(dragon_code(), boolean()) :: ansi() | nil
  def ansi_code(code, true) do
    case Map.fetch(fallback_map(), code) do
      {:ok, fallback} ->
        ansi_code(fallback, false)

      :error ->
        ansi_code(code, false)
    end
  end

  @spec parse(String.t, boolean()) :: String.t
  def parse(str, fallback \\ false) do
    Regex.replace(@dragon_code_rx, str, parse_replacer(fallback))
  end

  @spec parse_replacer(boolean()) :: ((String.t(), String.t(), String.t()) -> String.t())
  defp parse_replacer(fallback) do
    fn match, simple, xterm ->
      code = if simple == "", do: xterm, else: simple 
      ansi = ansi_code(code, fallback)
      starts = String.starts_with?(match, "[[")
      ends = String.ends_with?(match, "]]")
  
      cond do
        starts && ends -> 
          match
          |> String.replace("[[", "[")
          |> String.replace("]]", "]")
        starts && !ends -> "[" <> ansi
        !starts && ends -> ansi <> "]"
        true -> ansi
      end
    end
  end

  def escape(str, _fallback \\ false) do
    Regex.replace(@dragon_code_rx, str, &escape_replacer/3)
  end

  defp escape_replacer(match, _, _) do 
    starts = String.starts_with?(match, "[[")
    ends = String.starts_with?(match, "]]")
    if starts && ends do
      match
    else
      "[" <> match <> "]"
    end
  end
end
