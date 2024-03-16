-- SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
-- SPDX-FileCopyrightText:  Copyright 2024 András Kovács
-- SPDX-License-Identifier: MIT
--
-- Project:  1-billion-row-challenge
-- File:     Main.hs
-- Date:     15.Mar.2024
--
--------------------------------------------------------------------------------
{-# LANGUAGE BlockArguments #-}
{-# LANGUAGE MagicHash #-}
{-# LANGUAGE OverloadedStrings #-}
{-# LANGUAGE PatternSynonyms #-}
{-# LANGUAGE StrictData #-}
{-# LANGUAGE UnboxedTuples #-}
{-# LANGUAGE UnliftedDatatypes #-}
{-# LANGUAGE ViewPatterns #-}

module Main (main) where

import Control.Monad (when)
import Data.Bits (Bits (unsafeShiftL, (.|.)), (.&.))
import Data.ByteString.Char8 qualified as BS
import Data.ByteString.Internal qualified as BSI (
  ByteString (BS),
  unsafeCreate,
 )
import Data.Char (Char, ord)
import Data.Foldable (traverse_)
import Data.HashMap.Strict qualified as HM ()
import Data.List (sort, uncons)
import Data.Maybe (fromMaybe)
import Data.Primitive (
  Prim (
    alignmentOfType#,
    indexByteArray#,
    indexOffAddr#,
    readByteArray#,
    readOffAddr#,
    sizeOfType#,
    writeByteArray#,
    writeOffAddr#
  ),
  Ptr (Ptr),
 )
import Data.Primitive.PrimArray qualified as PA
import Data.Text.Encoding qualified as TE
import Data.Word (Word, Word8)
import Foreign.ForeignPtr (newForeignPtr_, withForeignPtr)
import Foreign.ForeignPtr.Unsafe (unsafeForeignPtrToPtr)
import GHC.Base (
  Addr#,
  IO (IO),
  Int (I#),
  Int#,
  RealWorld,
  Word (W#),
  Word#,
  addr2Int#,
  eqWord#,
  eqWord32#,
  eqWord8#,
  indexIntOffAddr#,
  indexWord32OffAddr#,
  indexWord8OffAddr#,
  indexWordOffAddr#,
  int2Addr#,
  int2Word#,
  isTrue#,
  plusAddr#,
  timesWord2#,
  uncheckedIShiftRL#,
  word2Int#,
  word32ToWord#,
  word8ToWord#,
  writeIntOffAddr#,
  xor#,
  (*#),
  (+#),
 )
import GHC.Float (roundDouble)
import GHC.IO (unsafeDupablePerformIO)
import System.Environment (getArgs)
import System.IO.MMap qualified as M
import Text.Printf qualified as TP (perror, printf)
import Prelude (
  Applicative (pure),
  Bool (False, True),
  Double,
  Eq ((==)),
  Foldable (null),
  Fractional ((/)),
  Maybe (Just, Nothing),
  Num (abs, signum, (*), (+), (-)),
  Ord (max, min, (<), (<=), (>=)),
  Semigroup ((<>)),
  Show (show),
  fromIntegral,
  head,
  otherwise,
  undefined,
  ($),
  (&&),
  (<$>),
 )

--------------------------------------------------------------------------------
-- Kovacshash
-- see https://discourse.haskell.org/t/one-billion-row-challenge-in-hs/8946/141
-- and https://gist.github.com/AndrasKovacs/e156ae66b8c28b1b84abe6b483ea20ec

tableSize :: Int
tableSize = 65536

mask :: Int
mask = tableSize - 1

salt :: Word
salt = 3032525626373534813

unW# :: Word -> Word#
unW# (W# x) = x

foldedMul :: Word# -> Word# -> Word#
foldedMul x y = case timesWord2# x y of (# hi, lo #) -> xor# hi lo

combine :: Word# -> Word# -> Word#
combine x y = foldedMul (xor# x y) 11400714819323198549##

goHash :: Addr# -> Int -> Word# -> Word#
goHash p l acc
  | l >= 8 =
      let w = indexWordOffAddr# p 0# in goHash (plusAddr# p 8#) (l - 8) (combine w acc)
  | l >= 4 =
      let w = indexWord32OffAddr# p 0#
       in goHash (plusAddr# p 4#) (l - 4) (combine (word32ToWord# w) acc)
  | l == 0 = acc
  | otherwise =
      let w = indexWord8OffAddr# p 0#
       in goHash (plusAddr# p 1#) (l - 1) (combine (word8ToWord# w) acc)

--------------------------------------------------------------------------------

-- Unsafe ByteString

data UnsafeByteString = MkUBS# !Int !Int !Int

lenUBS :: UnsafeByteString -> Int
lenUBS (MkUBS# a _ _) = a .&. 255

unpackUBS# :: UnsafeByteString -> (# (# Int#, Int#, Int#, Int# #) | (# Addr#, Int# #) #)
unpackUBS# (MkUBS# (I# a) (I# b) (I# c)) =
  let !(I# l) = I# a .&. 255
   in if I# l <= 23
        then (# (# l, uncheckedIShiftRL# a 8#, b, c #) | #)
        else (# | (# int2Addr# b, a #) #)

pattern ShortUBS :: Int -> Int -> Int -> Int -> UnsafeByteString
pattern ShortUBS len a b c <- (unpackUBS# -> (# (# I# -> len, I# -> a, I# -> b, I# -> c #) | #))
  where
    ShortUBS len a b c = MkUBS# (unsafeShiftL a 8 .|. len) b c

pattern LongUBS :: Ptr Word8 -> Int -> UnsafeByteString
pattern LongUBS p len <- (unpackUBS# -> (# | (# Ptr -> p, I# -> len #) #))
  where
    LongUBS (Ptr addr) len = MkUBS# len (I# (addr2Int# addr)) 0
{-# COMPLETE ShortUBS, LongUBS #-}

instance Eq UnsafeByteString where
  (==) (ShortUBS l a b c) (ShortUBS l' a' b' c') =
    l == l' && a == a' && b == b' && c == c'
  (==) (LongUBS (Ptr p) l) (LongUBS (Ptr p') l')
    | l == l' = isTrue# (goEqUBS p p' l)
    | otherwise = False
  (==) _ _ = False

goEqUBS :: Addr# -> Addr# -> Int -> Int#
goEqUBS p p' l
  | l >= 8 = case eqWord# (indexWordOffAddr# p 0#) (indexWordOffAddr# p' 0#) of
      1# -> goEqUBS (plusAddr# p 8#) (plusAddr# p' 8#) (l - 8)
      _ -> 0#
  | l >= 4 = case eqWord32# (indexWord32OffAddr# p 0#) (indexWord32OffAddr# p' 0#) of
      1# -> goEqUBS (plusAddr# p 4#) (plusAddr# p' 4#) (l - 4)
      _ -> 0#
  | l == 0 = 1#
  | otherwise = case eqWord8# (indexWord8OffAddr# p 0#) (indexWord8OffAddr# p' 0#) of
      1# -> goEqUBS (plusAddr# p 1#) (plusAddr# p' 1#) (l - 1)
      _ -> 0#

unsafeIShiftRL :: Int -> Int -> Int
unsafeIShiftRL (I# x) (I# y) = I# (uncheckedIShiftRL# x y)

fromByteStringUBS :: BS.ByteString -> UnsafeByteString
fromByteStringUBS (BSI.BS fp l) =
  let
    !(Ptr p) = unsafeForeignPtrToPtr fp
    index i = I# (indexIntOffAddr# p i)
    mask l = unsafeIShiftRL ((-1) :: Int) (64 - unsafeShiftL l 3)
   in
    if l <= 8
      then ShortUBS l 0 0 (index 0# .&. mask l)
      else
        if l <= 16
          then ShortUBS l 0 (index 1# .&. mask (l - 8)) (index 0#)
          else
            if l <= 23
              then ShortUBS l (index 2# .&. mask (l - 16)) (index 1#) (index 0#)
              else LongUBS (Ptr p) l

toByteStringUBS :: UnsafeByteString -> BS.ByteString
toByteStringUBS (ShortUBS l (I# a) (I# b) (I# c)) = BSI.unsafeCreate l \(Ptr p) -> IO \s ->
  case writeIntOffAddr# p 0# c s of
    s -> case writeIntOffAddr# p 1# b s of
      s -> case writeIntOffAddr# p 2# a s of
        s -> (# s, () #)
toByteStringUBS (LongUBS p l) = BSI.BS (unsafeDupablePerformIO (newForeignPtr_ p)) l

instance Show UnsafeByteString where
  show (MkUBS# a b c) = show (a, b, c)

hashUBS :: UnsafeByteString -> Int
hashUBS (ShortUBS _ (I# a) (I# b) (I# c)) =
  I# (word2Int# (int2Word# a `combine` int2Word# b `combine` int2Word# c))
hashUBS (LongUBS (Ptr addr) l) =
  I# (word2Int# (goHash addr l (unW# salt)))

emptyUBS :: UnsafeByteString
emptyUBS = ShortUBS 0 0 0 0

isEmptyUBS :: UnsafeByteString -> Bool
isEmptyUBS x = lenUBS x == 0

data Entry = Entry
  { _station :: {-# UNPACK #-} !Station
  , _temperature :: !Int
  }
  deriving (Show)

type Station = UnsafeByteString

mkStation :: BS.ByteString -> Station
mkStation = fromByteStringUBS

data Quartet = Quartet
  { _min :: !Int
  , _total :: !Int
  , _cnt :: !Int
  , _max :: !Int
  }
  deriving (Eq)

mkQuartet :: Int -> Quartet
mkQuartet x = Quartet x x 1 x

updateQuartet :: Int -> Quartet -> Quartet
updateQuartet x (Quartet a b c d) = Quartet (min a x) (b + x) (c + 1) (max d x)

instance Semigroup Quartet where
  Quartet a b c d <> Quartet a' b' c' d' =
    Quartet (min a a') (b + b') (c + c') (max d d')

data Row = Row {-# UNPACK #-} !Station {-# UNPACK #-} !Quartet

instance Prim Row where
  sizeOfType# _ = 7# *# 8#
  alignmentOfType# _ = 8#
  indexByteArray# = undefined
  readByteArray# mba i s0 = (# s7, Row (MkUBS# a b c) (Quartet d e f g) #)
   where
    !(# s1, a :: Int #) = readByteArray# mba (7# *# i) s0
    !(# s2, b :: Int #) = readByteArray# mba (7# *# i +# 1#) s1
    !(# s3, c :: Int #) = readByteArray# mba (7# *# i +# 2#) s2
    !(# s4, d :: Int #) = readByteArray# mba (7# *# i +# 3#) s3
    !(# s5, e :: Int #) = readByteArray# mba (7# *# i +# 4#) s4
    !(# s6, f :: Int #) = readByteArray# mba (7# *# i +# 5#) s5
    !(# s7, g :: Int #) = readByteArray# mba (7# *# i +# 6#) s6
  {-# INLINE readByteArray# #-}
  writeByteArray# mba i (Row (MkUBS# a b c) (Quartet d e f g)) s0 = s7
   where
    s1 = writeByteArray# mba (7# *# i) a s0
    s2 = writeByteArray# mba (7# *# i +# 1#) b s1
    s3 = writeByteArray# mba (7# *# i +# 2#) c s2
    s4 = writeByteArray# mba (7# *# i +# 3#) d s3
    s5 = writeByteArray# mba (7# *# i +# 4#) e s4
    s6 = writeByteArray# mba (7# *# i +# 5#) f s5
    s7 = writeByteArray# mba (7# *# i +# 6#) g s6
  {-# INLINE writeByteArray# #-}
  indexOffAddr# = undefined
  readOffAddr# = undefined
  writeOffAddr# = undefined

data Table = MkTable !(PA.MutablePrimArray RealWorld Row)

insert :: Table -> Entry -> IO ()
insert (MkTable arr) (Entry name t) = do
  let i0 =
        -- traceShowId $
        hashUBS name .&. mask
  Row s q <- PA.readPrimArray arr i0
  if isEmptyUBS s
    then -- trace "done1" $
      PA.writePrimArray arr i0 (Row name (mkQuartet t))
    else
      if s == name
        then -- trace "done2" $
          PA.writePrimArray arr i0 (Row name (updateQuartet t q))
        else go ((i0 + 1) .&. mask)
 where
  go i =
    -- \| traceShow i True
    do
      Row s q <- PA.readPrimArray arr i
      if isEmptyUBS s
        then -- trace "done3" $
          PA.writePrimArray arr i (Row name (mkQuartet t))
        else
          if s == name
            then -- trace "done4"
              PA.writePrimArray arr i (Row name (updateQuartet t q))
            else go ((i + 1) .&. mask)

newTable :: Int -> IO Table
newTable l = do
  arr <- PA.newPrimArray l
  PA.setPrimArray arr 0 l (Row emptyUBS (Quartet 0 0 0 0))
  pure (MkTable arr)

tableToList :: Table -> IO [Row]
tableToList (MkTable arr) = do
  sz <- PA.getSizeofMutablePrimArray arr
  go sz 0
 where
  go :: Int -> Int -> IO [Row]
  go sz i
    | i < sz = do
        r@(Row ubs _) <- PA.readPrimArray arr i
        if isEmptyUBS ubs
          then go sz (i + 1)
          else do
            !tl <- go sz (i + 1)
            pure (r : tl)
    | otherwise = pure []

--------------------------------------------------------------------------------
-- my part

parse ::
  BS.ByteString ->
  Table ->
  IO Table
parse (BS.null -> True) acc = pure acc
parse content accMap = do
  let (!stationName', !rest') = BS.break (== ';') content
  let stationName = mkStation stationName'
  let !rest2 = BS.drop 1 rest'
  let (!temp, !rest) = case BS.uncons rest2 of
        Nothing -> (0, rest2)
        Just ('-', !rest2') -> parseNegTemp rest2'
        Just (ch1, !rest3) -> parseTemp ch1 rest3
  insert accMap (Entry stationName temp)
  parse (BS.drop 1 rest) accMap

{-# INLINE parseNegTemp #-}
parseNegTemp :: BS.ByteString -> (Int, BS.ByteString)
parseNegTemp !rest = case BS.uncons rest of
  Nothing -> (0, rest)
  Just (!ch1, !rest3) -> case BS.uncons rest3 of
    Nothing -> (0, rest3)
    Just ('.', !rest4) -> case BS.uncons rest4 of
      Nothing -> (0, rest4)
      Just (!ch2, !rest5) -> (528 - 10 * ord ch1 - ord ch2, rest5)
    Just (!ch2, !rest4) -> case BS.uncons rest4 of
      Nothing -> (0, rest4)
      -- Must be '.'
      Just (_, !rest5) -> case BS.uncons rest5 of
        Nothing -> (0, rest5)
        Just (!ch3, !rest6) -> (5328 - 100 * ord ch1 - 10 * ord ch2 - ord ch3, rest6)

{-# INLINE parseTemp #-}
parseTemp :: Char -> BS.ByteString -> (Int, BS.ByteString)
parseTemp !ch1 !rest = case BS.uncons rest of
  Nothing -> (0, rest)
  Just ('.', !rest4) -> case BS.uncons rest4 of
    Nothing -> (0, rest4)
    Just (!ch2, !rest5) -> (10 * ord ch1 + ord ch2 - 528, rest5)
  Just (!ch2, !rest4) -> case BS.uncons rest4 of
    Nothing -> (0, rest4)
    -- Must be '.'
    Just (_, !rest5) -> case BS.uncons rest5 of
      Nothing -> (0, rest5)
      Just (!ch3, !rest6) -> (100 * ord ch1 + 10 * ord ch2 + ord ch3 - 5328, rest6)

main :: IO ()
main = do
  args <- getArgs
  when (null args) $ TP.perror "Error: no data file to read given! Exiting."
  content@(BSI.BS fp _) <- M.mmapFileByteString (head args) Nothing
  withForeignPtr fp $ \_ -> do
    hm <- newTable tableSize
    ma <- parse content hm

    keys' <- tableToList ma
    let keys = sort $ (\(Row n (Quartet a b c d)) -> (toByteStringUBS n, a, b, c, d)) <$> keys'
    TP.printf "{"
    let ((e1, tMin1, sum1, count1, tMax1), els) = fromMaybe (("", 0, 0, 0, 0), []) $ uncons keys
    let name1 = TE.decodeUtf8 e1
    let mean1 :: Double = fromIntegral sum1 / fromIntegral count1
    TP.printf
      "%s=%.1f/%.1f/%.1f"
      name1
      (roundJava $ fromIntegral tMin1)
      (roundJava mean1)
      (roundJava $ fromIntegral tMax1)
    traverse_
      ( \(k, tMin, sum, count, tMax) ->
          do
            let name = TE.decodeUtf8 k
            let mean :: Double = fromIntegral sum / fromIntegral count
            TP.printf
              ", %s=%.1f/%.1f/%.1f"
              name
              (roundJava $ fromIntegral tMin)
              (roundJava mean)
              (roundJava $ fromIntegral tMax)
      )
      els
    TP.printf "}\n"

{-# INLINE roundJava #-}
roundJava :: Double -> Double
roundJava y =
  let r :: Double = fromIntegral (roundDouble y :: Int)
   in case y of
        x
          | x < 0.0 && r - x == 0.5 -> r / 10.0
          | abs (x - r) >= 0.5 -> (r + signum y) / 10.0
          | otherwise -> r / 10.0
