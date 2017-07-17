-- validate that tbl is a table and that fn is a function.
--
-- params:
--   tbl = value to check if is a table
--   fn = value to check if is a function
--
-- returns:
--   true if the tbl is a table and fn is a function, false otherwise
function is_valid_params(tbl, fn)
  return type(tbl) == "table" and type(fn) == "function"
end

-- determine if a table is a list (has a length) or not
--
-- params:
--   tbl = table to check if is a list
--
-- returns:
--   true if tbl has length > 0
function is_list(tbl)
  return #list > 0
end

function sum_reducer(sum, current)
  return sum + current
end

-- for the sake of the documentation in this module, 'list' refers to tables
-- being used strictly as lists of data, like {1, 2, 3} while 'map' refers to
-- using tables as key/value storage, like {one = 1, two = 2}.
--
-- with 'map' types, due to being implemented in Go, the order of keys is not
-- guaranteed. So anything that iterates/builds new tables from maps is not
-- guaranteed to preserve order -- and functions that are designed to be
-- sequential, like find/take/drop, have undefined results.
local fn = {
  -- each iterates all values in the table and calls the action function on each
  -- value.
  --
  -- for lists, the function is expected to have the signature:
  --   action(value: any, index: number)
  -- for maps, the function is expected to have the signature:
  --   action(value: any, key: any)
  --
  -- params:
  --   tbl = the table to iterate
  --   action = the function to run on each element or key/value pair of the
  --            table.
  each = function(tbl, action)
    if is_valid_params(tbl, action) then
      if is_list(tbl) then
        for i = 1, #list do
          action(tbl[i], i)
        end
      else
        for k, v in pairs(tbl) do
          action(v, k)
        end
      end
    end

    return nil
  end,

  -- map iterates over the values in the table, sending each one to the mapper
  -- function and building a new table which it returns.
  --
  -- for lists, the function is expected to have the signature:
  --   mapper(value: any, index: number): any
  -- for maps, the function is expected to have the signature:
  --   mapper(value: any, key: any): any, any
  --   NOTE: return value is the new key/value pair.
  --
  -- params:
  --   tbl = table of values that will be iterated
  --   mapper = function that transforms the values to create a new table
  --
  -- returns:
  --   a new table with the mapper applied to all values.
  map = function(tbl, mapper)
    if is_valid_params(tbl, mapper) then
      local new_list = {}

      if is_list(tbl) then
        for i = 1, #list do
          table.insert(new_list, mapper(tbl[i], i))
        end
      else
        for k, v in pairs(tbl) do
          nk, nv = mapper(v, k)
          new_list[nk] = nv
        end
      end

      return new_list
    end

    return {}
  end,

  -- reduce starts with an initial value and then calls the reducer with the
  -- value and the current item in the table and expects the new value in
  -- return, which it uses on the next iteration.
  --
  -- for lists, the function is expected to have the signature:
  --   reducer(sum_value: any, current_value: any, index: number): any
  -- for maps, the function is expected to have the signature:
  --   reducer(sum_value: any, current_value: any, key: any): any
  --
  -- params:
  --   tbl = the table to reduce
  --   val = the inital value to seed the reducer function with
  --   reducer = the reduction function that will take the sum value, the
  --             current value, it's key (index or actual key) and expect the
  --             the new sum value in return which is passed to the next call
  reduce = function(tbl, val, reducer)
    if is_valid_params(tbl, reducer) then
      if is_list(tbl) then
        for i = 1, #list do
          val = reducer(val, list[i], i)
        end
      else
        for k, v in pairs(tbl) do
          val = reducer(val, v, k)
        end
      end
    end

    return val
  end,

  -- sum is a shorthand reduction function for lists of numbers, it's designed
  -- to only handle tables that are made up of numbers, or have only key/value
  -- pairs where the value is a number.
  --
  -- params:
  --   tbl = the table with numbers that need to be summed
  --
  -- returns:
  --   the sum of all values in the input table
  sum = function(tbl)
    return fn.reduce(tbl, 0, sum_reducer)
  end,

  -- filter will iterate over the table and only keep values that pass the
  -- given filter function.
  --
  -- for lists, the function is expected to have the signature:
  --   filter(value: any, index: number): boolean
  -- for maps, the function is expected to have the signature:
  --   filter(value: any, key: any): boolean
  --
  -- params:
  --   tbl = the table whose values will be filtered
  --   filter = a test function returning true for desired values
  --
  -- returns:
  --   a new table containing only those values from the original list that
  --   passed the filter
  filter = function(tbl, filter)
    if is_valid_params(tbl, filter) then
      local new_list = {}
      if is_list(tbl) then
        for i = 1, #list do
          if filter(tbl[i], i) then
            table.insert(new_list, list[i])
          end
        end
      else
        for k, v in pairs(tbl) do
          if filter(v, k) then
            new_list[k] = v
          end
        end
      end

      return new_list
    end

    return list
  end,

  -- find searches the table and returns the first value it comes across that
  -- passes the finder method provided. this method stops iteration as soon as
  -- a value is found.
  --
  -- for lists, the function is expected to have the signature:
  --   finder(value: any, index: number): boolean
  -- for maps, the function is expected to have the signature:
  --   finder(value: any, key: any): boolean
  --
  -- params:
  --   tbl = the table to search through for a desired value
  --   finder = a function used to determine if a given value is desired
  --
  -- returns:
  --   returns the value found if one passes the finder function, otherwise it
  --   returns nil to signify no value was found
  find = function(tbl, finder)
    if is_valid_params(tbl, finder) then
      if is_list(tbl) then
        for i = 1, #list do
          if finder(tbl[i], i) then
            return list[i]
          end
        end
      else
        for k, v in pairs(tbl) do
          if finder(v, k) then
            return v
          end
        end
      end
    end

    return nil
  end,

  -- any takes a test function and returns true if any value in the list passes
  -- the given test function. any is similar to find except that any only
  -- returns a boolean.
  --
  -- for lists, the function is expected to have the signature:
  --   tester(value: any, index: number): boolean
  -- for maps, the function is expected to have the signature:
  --   tester(value: any, key: any): boolean
  --
  -- params:
  --   tbl = table containing the data to test with the tester function
  --   tester = function used to determine if the table contains a value that
  --            matches the criteria
  --
  -- returns:
  --   true if at least one value in the list passes the tester function,
  --   otherwise false.
  any = function(tbl, tester)
    if is_valid_params(tbl, tester) then
      if is_list(tbl) then
        for i = 1, #list do
          if tester(tbl[i], i) then
            return true
          end
        end
      else
        for k, v in pairs(tbl) do
          if tester(v, k) then
            return true
          end
        end
      end
    end

    return false
  end,

  -- all is similar to any, except that it verifies if all the values in a
  -- list pass a tester function.
  --
  -- an easy way to see if the list contains all truth-y values:
  --   fn.all(list, fn.identity)
  --
  -- for lists, the function is expected to have the signature:
  --   tester(value: any, index: number): boolean
  -- for maps, the function is expected to have the signature:
  --   tester(value: any, key: any): boolean
  --
  -- params:
  --   tbl = the table to test all values of
  --   tester = the function to determine if all the values in the given table
  --            meet the criteria of
  --
  -- returns:
  --   true if all values in the table pass the tester function, or false
  --   otherwise
  all = function(tbl, tester)
    if is_valid_params(tbl, tester) then
      if is_list(tbl) then
        for i = 1, #list do
          if not tester(tbl[i], i) then
            return false
          end
        end
      else
        for k, v in pairs(tbl) do
          if not tester(v, k) then
            return false
          end
        end
      end
    end

    return true
  end,

  -- take will return a new table containing only the first n values from the
  -- given table.
  --
  -- WARN: while this method (like all other fn methods) work on map tables,
  --       the behavior is undefined.
  --
  -- NOTE: this method is optimized for lists, but not maps
  --
  -- params:
  --   tbl = table to take the first n values from
  --   amount = number of values to take from the given table
  --
  -- returns:
  --   new table containing the first n values from the given table
  take = function(tbl, amount)
    local new_list = {}
    -- no fn required, so we used ident as a 'always true' value
    if amount > 0 and is_valid_params(tbl, fn.ident) then
      if is_list(tbl) then
        for i = 1, amount do
          if i <= #list then
            table.insert(new_list, list[i])
          end
        end
      else
        local iterations = 0
        for k, v in pairs(tbl) do
          new_list[k] = v
          iterations = iterations + 1
          if iterations >= amount then
            return new_list
          end
        end
      end
    end

    return new_list
  end,

  -- drop is similar to take, except that it ignores the first n values of the
  -- given table and then returns all the values after it.
  --
  -- WARN: while this method (like all other fn methods) work on map tables,
  --       the behavior is undefined.
  --
  -- NOTE: this method is optimized for lists, but not maps
  --
  -- params:
  --   tbl = the table whose first n values will be ignored
  --   amount = the number of values to ignore at the beginning of tbl
  --
  -- returns:
  --   a new list containing all values in the list after the first n have been
  --   skipped.
  drop = function(tbl, amount)
    local new_list = {}
    -- no fn required, so we used ident as a 'always true' value
    if is_valid_params(tbl, fn.ident) then
      if is_list(tbl) then
        for i = (amount + 1), #list do
          table.insert(new_list, list[i])
        end
      else
        local iterations = 0
        for k, v in pairs(tbl) do
          if iterations < amount then
            iterations = iterations + 1
          else
            new_list[k] = v
          end
        end
      end
    end

    return new_list
  end,

  -- identity is the identity function, it returns whatever value it is given.
  --
  -- params:
  --   x = the value to return
  --
  -- returns:
  --   the value of it's first parameter
  identity = function(x)
    return x
  end,

  -- odd is a numeric helper method, used to determine if a number is odd
  --
  -- params:
  --   x = number to test for odd-ness
  --
  -- returns:
  --   true if the number is not divisble by 2, false otherwise
  odd = function(x)
    return x % 2 ~= 0
  end,

  -- even is a numeric helper method, used to determine if a number is even
  --
  -- params:
  --   x = number to test for even-ness
  --
  -- returns:
  --   true if the number is divisible by 2, false otherwise
  even = function(x)
    return x % 2 == 0
  end
}

-- ident is an alias for identity, for the lazier
fn.ident = fn.identity

-- Value is a special type that lets you use functional methods as member
-- functions of a list, you use it like this:
--
--   fn.value(list):map(...):filter(...):drop(...):value()
--
-- where fn.value constructs a Value from a given table and Value:value()
-- returns the wrapped list.
--
-- the purpose of this type is for chaining functional methods, the chained
-- versions of the functional methods that return new tables return those
-- tables in the form of a Value so they, too, can be chained.
--
--   fn.value{1, 2, 3, 4}:filter(fn.odd):map(function(x) return x * x end):value()
--    => {1, 9}
local Value = {}

-- Value:value returns the wrapped table
--
-- returns:
--   the table value wrapped by the value
function Value:value()
  return self._table
end

-- Value:map generates a new Value by calling the mapper on all values of the
-- wrapped list
--
-- see: fn.map
--
-- params:
--   mapper = the transformation function used to build a new Value
--
-- returns:
--   a Value containing the new table genereated from mapping all values in the
--   wrapped table
function Value:map(mapper)
  local new_val = fn.map(self._table, mapper)

  return fn.value(new_val)
end

-- Value:each iterates all values of the wrapped table and calls action on them
--
-- see: fn.each
--
-- params:
--   action = the action to perform on each value of the wrapped table
--
-- returns:
--   self to enable chaining more events on
function Value:each(action)
  fn.each(self._table, action)

  return self
end

-- Value:reduce will reduce all values of the wrapped table, starting with
-- the given initial value.
--
-- see: fn.reduce
--
-- params:
--   value = the seed value to start the reduction of the wrapped table
--   reducer = the reduction function that takes the current sum value and
--             current table value and returns the new sum value
--
-- returns:
--   the final reduced result after calling the reduction sequentially on all
--   values
function Value:reduce(value, reducer)
  return fn.reduce(self._table, value, reducer)
end

-- Value:sum is a shortcut reduction on numeric tables that sums the numeric
-- values within the wrapped table.
--
-- see: fn.sum
--
-- returns:
--   the total of all numeric values in the given table
function Value:sum()
  return fn.sum(self._table)
end

-- Value:filter will filter the wrapped table's values and return a new Value
-- containing all values that passed the filter
--
-- see: fn.filter
--
-- params:
--   filter = the filter function used to test values
--
-- returns:
--   a new Value wrapping a table of all values from the wrapped table that
--   passed the filter
function Value:filter(filter)
  local filtered = fn.filter(self._table, filter)

  return fn.value(filtered)
end

-- Value:find searches the wrapped table for a value that matches the finder
-- and returns the first one that it sees.
--
-- see: fn.find
--
-- params:
--   finder = the function used to test values to determine if it is the value
--            being sought
--
-- returns:
--   if a value passes the finder then that value is returned, otherwise nil
--   is returned
function Value:find(finder)
  return fn.find(self._table, finder)
end

-- Value:any determines if a single value in the wrapped table matches the given
-- tester.
--
-- see: fn.any
--
-- params:
--   tester = the function to test values against looking for a pass
--
-- returns:
--   true if at least one value in the wrapped table passes the tester, or
--   false otherwise
function Value:any(tester)
  return fn.any(self._table, tester)
end

-- Value:all determines if all values in the wrapped table match the given
-- tester
--
-- see: fn.all
--
-- params:
--   tester = the function to test values against looking for a pass
--
-- returns:
--   true if every value in the wrapped table passes the tester, or false
--   otherwise
function Value:all(tester)
  return fn.any(self._table, tester)
end

-- Value:take will return the first n values in the wrapped list as a new Value
--
-- see: fn.take
--
-- params:
--   amount = the number of values to keep from the wrapped table
--
-- returns:
--   a new Value wrapping a table of the first n values in the wrapped table.
function Value:take(amount)
  local value = fn.take(self._table, amount)

  return fn.value(value)
end

-- Value:drop will return all values after the first n values in the wrapped
-- table.
--
-- see: fn.drop
--
-- params:
--   amount = the number of elements to skip over in the wrapped table
--
-- returns:
--   a new Value wrapping a table containing all elements after the first n
--   in the wrapped table
function Value:drop(amount)
  local value = fn.drop(self._table, amount)

  return fn.value(value)
end

-- value will return a new Value wrapping the given table value
--
-- params:
--   tbl = the table to wrap
--
-- returns:
--   a new Value wrapping the given table that functional methods can be
--   used on
function fn.value(tbl)
  local val = {_table = list}
  setmetatable(val, {__index = Value})

  return val
end

-- return the module
return fn
